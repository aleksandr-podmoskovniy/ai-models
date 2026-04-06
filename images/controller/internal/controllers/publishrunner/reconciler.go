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
	"fmt"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsession"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var operation corev1.ConfigMap
	if err := r.client.Get(ctx, req.NamespacedName, &operation); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !IsManagedConfigMap(&operation) {
		return ctrl.Result{}, nil
	}

	request, err := RequestFromConfigMap(&operation)
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, &operation, err.Error())
	}

	status := StatusFromConfigMap(&operation)
	if err := validatePersistedStatus(&operation, status); err != nil {
		return ctrl.Result{}, r.failOperation(ctx, &operation, err.Error())
	}
	if publicationdomain.IsTerminalOperationPhase(publicationdomain.OperationPhase(status.Phase)) {
		return ctrl.Result{}, nil
	}

	mode, err := publicationapp.StartPublication(publicationapp.StartPublicationInput{
		SourceType:   request.Spec.Source.Type,
		Upload:       request.Spec.Source.Upload,
		RuntimeHints: request.Spec.RuntimeHints,
	})
	if err != nil {
		return ctrl.Result{}, r.failOperation(ctx, &operation, err.Error())
	}

	switch mode {
	case publicationapp.ExecutionModeUpload:
		return r.reconcileUploadSession(ctx, &operation, request, status)
	case publicationapp.ExecutionModeSourceWorker:
		return r.reconcileSourceWorker(ctx, &operation, request, status)
	default:
		return ctrl.Result{}, r.failOperation(ctx, &operation, "publication operation entered an unsupported execution mode")
	}
}

func newReconciler(client client.Client, scheme *runtime.Scheme, options Options) (*Reconciler, error) {
	sourceWorkers, err := sourceworker.NewService(client, scheme, options.PublishPod.sourceWorkerOptions())
	if err != nil {
		return nil, err
	}
	uploadSessions, err := uploadsession.NewService(client, scheme, options.PublishPod.uploadSessionOptions())
	if err != nil {
		return nil, err
	}

	return &Reconciler{
		client:         client,
		options:        options,
		sourceWorkers:  sourceWorkers,
		uploadSessions: uploadSessions,
	}, nil
}

func (r *Reconciler) failOperation(ctx context.Context, operation *corev1.ConfigMap, message string) error {
	if err := SetFailed(operation, message); err != nil {
		return err
	}

	return r.client.Update(ctx, operation)
}

func (r *Reconciler) applySourceWorkerDecision(
	ctx context.Context,
	operation *corev1.ConfigMap,
	decision publicationdomain.SourceWorkerDecision,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	return r.applyOperationDecision(
		ctx,
		operation,
		decision.FailMessage,
		decision.DeleteWorker,
		deleteFn,
		func(operation *corev1.ConfigMap) (bool, error) {
			if decision.Success != nil {
				if err := SetSucceeded(operation, publicationports.Result{
					Snapshot:      decision.Success.Snapshot,
					CleanupHandle: decision.Success.CleanupHandle,
				}); err != nil {
					return false, err
				}
				return true, nil
			}
			if decision.PersistRunning {
				if err := SetRunning(operation, decision.RunningWorker); err != nil {
					return false, err
				}
				return true, nil
			}
			return false, nil
		},
		decision.RequeueAfter,
	)
}

func (r *Reconciler) applyUploadSessionDecision(
	ctx context.Context,
	operation *corev1.ConfigMap,
	decision publicationdomain.UploadSessionDecision,
	deleteFn func(context.Context) error,
) (ctrl.Result, error) {
	return r.applyOperationDecision(
		ctx,
		operation,
		decision.FailMessage,
		decision.DeleteSession,
		deleteFn,
		func(operation *corev1.ConfigMap) (bool, error) {
			mutated := false
			if decision.Success != nil {
				if err := SetSucceeded(operation, publicationports.Result{
					Snapshot:      decision.Success.Snapshot,
					CleanupHandle: decision.Success.CleanupHandle,
				}); err != nil {
					return false, err
				}
				mutated = true
			}
			if decision.PersistRunning {
				if err := SetRunning(operation, decision.RunningWorker); err != nil {
					return false, err
				}
				mutated = true
			}
			if decision.PersistUpload {
				if err := SetUploadReady(operation, *decision.UploadStatus); err != nil {
					return false, err
				}
				mutated = true
			}
			return mutated, nil
		},
		decision.RequeueAfter,
	)
}

func (r *Reconciler) applyOperationDecision(
	ctx context.Context,
	operation *corev1.ConfigMap,
	failMessage string,
	deleteOwned bool,
	deleteFn func(context.Context) error,
	mutate func(*corev1.ConfigMap) (bool, error),
	requeueAfter time.Duration,
) (ctrl.Result, error) {
	if deleteOwned {
		if err := deleteFn(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}
	if failMessage != "" {
		return ctrl.Result{}, r.failOperation(ctx, operation, failMessage)
	}
	mutated, err := mutate(operation)
	if err != nil {
		return ctrl.Result{}, err
	}
	if mutated {
		if err := r.client.Update(ctx, operation); err != nil {
			return ctrl.Result{}, err
		}
	}
	if requeueAfter > 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	return ctrl.Result{}, nil
}

func operationStatusView(status publicationports.Status) publicationdomain.OperationStatusView {
	return publicationdomain.OperationStatusView{
		Phase:      publicationdomain.OperationPhase(status.Phase),
		WorkerName: status.WorkerName,
	}
}

func validatePersistedStatus(operation *corev1.ConfigMap, status publicationports.Status) error {
	switch status.Phase {
	case publicationports.PhasePending, publicationports.PhaseFailed:
		return nil
	case publicationports.PhaseRunning:
		if _, err := UploadStatusFromConfigMap(operation); err != nil {
			return fmt.Errorf("publication operation running state has invalid persisted upload payload: %w", err)
		}
		return nil
	case publicationports.PhaseSucceeded:
		if _, err := ResultFromConfigMap(operation); err != nil {
			return fmt.Errorf("publication operation succeeded without a valid persisted result: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("publication operation has unsupported persisted phase %q", status.Phase)
	}
}
