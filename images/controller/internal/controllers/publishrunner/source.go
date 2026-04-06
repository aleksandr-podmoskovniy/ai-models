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

const sourceWorkerResultPollInterval = time.Second

func (r *Reconciler) reconcileSourceWorker(
	ctx context.Context,
	operation *corev1.ConfigMap,
	request publicationports.Request,
	status publicationports.Status,
) (ctrl.Result, error) {
	worker, created, err := r.sourceWorkers.GetOrCreate(ctx, operation, publicationports.OperationContext{
		Request:            request,
		OperationName:      operation.Name,
		OperationNamespace: operation.Namespace,
	})
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}
	if created {
		decision, err := publicationdomain.ObserveSourceWorker(publicationdomain.SourceWorkerObservation{
			Current:    operationStatusView(status),
			WorkerName: worker.Name,
			Created:    true,
			State:      publicationdomain.RuntimeStateRunning,
		})
		if err != nil {
			return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
		}
		return r.applySourceWorkerDecision(ctx, operation, decision, worker.Delete)
	}

	observation, err := r.buildSourceWorkerObservation(operation, request, status, worker)
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}

	decision, err := publicationdomain.ObserveSourceWorker(observation)
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, operation, err.Error())
	}

	return r.applySourceWorkerDecision(ctx, operation, decision, worker.Delete)
}

func (r *Reconciler) buildSourceWorkerObservation(
	operation *corev1.ConfigMap,
	request publicationports.Request,
	status publicationports.Status,
	worker *publicationports.SourceWorkerHandle,
) (publicationdomain.SourceWorkerObservation, error) {
	observation := publicationdomain.SourceWorkerObservation{
		Current:    operationStatusView(status),
		WorkerName: worker.Name,
		State:      publicationdomain.RuntimeStateRunning,
	}

	switch {
	case worker.IsComplete():
		rawResult := WorkerResultFromConfigMap(operation)
		if rawResult == "" {
			message := workerFailureMessage(operation, "")
			if message != "" {
				observation.State = publicationdomain.RuntimeStateFailed
				observation.Failure = message
				return observation, nil
			}
			observation.State = publicationdomain.RuntimeStateAwaitingResult
			observation.RequeueAfter = sourceWorkerResultPollInterval
			return observation, nil
		}
		success, err := publicationSuccessFromWorkerResult(rawResult, request)
		if err != nil {
			return publicationdomain.SourceWorkerObservation{}, err
		}
		observation.State = publicationdomain.RuntimeStateSucceeded
		observation.Success = success
	case worker.IsFailed():
		observation.State = publicationdomain.RuntimeStateFailed
		observation.Failure = workerFailureMessage(operation, "publication worker pod failed")
	}

	return observation, nil
}
