/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package publishrunner

import (
	"context"
	"time"

	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *Reconciler) reconcileUploadSession(
	ctx context.Context,
	operation *corev1.ConfigMap,
	request publicationports.Request,
	status publicationports.Status,
) (ctrl.Result, error) {
	session, created, err := r.uploadSessions.GetOrCreate(ctx, operation, publicationports.OperationContext{
		Request:            request,
		OperationName:      operation.Name,
		OperationNamespace: operation.Namespace,
	})
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}
	if session == nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, "upload session worker pod is missing")
	}

	observation, err := r.buildUploadSessionObservation(operation, request, status, session, created)
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}

	decision, err := publicationdomain.ObserveUploadSession(observation)
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}

	return r.applyUploadSessionDecision(ctx, operation, decision, session.Delete)
}

func (r *Reconciler) buildUploadSessionObservation(
	operation *corev1.ConfigMap,
	request publicationports.Request,
	status publicationports.Status,
	session *publicationports.UploadSessionHandle,
	created bool,
) (publicationdomain.UploadSessionObservation, error) {
	currentUpload, err := UploadStatusFromConfigMap(operation)
	if err != nil {
		return publicationdomain.UploadSessionObservation{}, err
	}
	observation := publicationdomain.UploadSessionObservation{
		Current:       operationStatusView(status),
		WorkerName:    session.WorkerName,
		Created:       created,
		State:         publicationdomain.RuntimeStateRunning,
		CurrentUpload: currentUpload,
	}

	switch {
	case session.IsComplete():
		rawResult := WorkerResultFromConfigMap(operation)
		if rawResult == "" {
			observation.State = publicationdomain.RuntimeStateFailed
			observation.Failure = workerFailureMessage(operation, "upload session completed without publication result")
			return observation, nil
		}
		success, err := publicationSuccessFromWorkerResult(rawResult, request)
		if err != nil {
			return publicationdomain.UploadSessionObservation{}, err
		}
		observation.State = publicationdomain.RuntimeStateSucceeded
		observation.Success = success
	case session.IsFailed():
		observation.State = publicationdomain.RuntimeStateFailed
		observation.Failure = workerFailureMessage(operation, "upload session worker pod failed")
	default:
		observation.UploadStatus = &session.UploadStatus
		if session.UploadStatus.ExpiresAt != nil {
			now := time.Now().UTC()
			if !session.UploadStatus.ExpiresAt.Time.After(now) {
				observation.Expired = true
				return observation, nil
			}
			observation.RequeueAfter = time.Until(session.UploadStatus.ExpiresAt.Time)
		}
	}

	return observation, nil
}
