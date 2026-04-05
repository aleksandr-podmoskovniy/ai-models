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

package publicationops

import (
	"context"

	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	corev1 "k8s.io/api/core/v1"
)

func (r *Reconciler) failOperation(ctx context.Context, operation *corev1.ConfigMap, message string) error {
	if err := SetFailed(operation, message); err != nil {
		return err
	}

	return r.client.Update(ctx, operation)
}

func (r *Reconciler) persistSourceWorkerDecision(
	ctx context.Context,
	operation *corev1.ConfigMap,
	decision publicationdomain.SourceWorkerDecision,
) error {
	if decision.Success != nil {
		if err := SetSucceeded(operation, publicationports.Result{
			Snapshot:      decision.Success.Snapshot,
			CleanupHandle: decision.Success.CleanupHandle,
		}); err != nil {
			return err
		}
		return r.client.Update(ctx, operation)
	}
	if decision.PersistRunning {
		if err := SetRunning(operation, decision.RunningWorker); err != nil {
			return err
		}
		return r.client.Update(ctx, operation)
	}

	return nil
}

func (r *Reconciler) persistUploadSessionDecision(
	ctx context.Context,
	operation *corev1.ConfigMap,
	decision publicationdomain.UploadSessionDecision,
) error {
	mutated := false
	if decision.Success != nil {
		if err := SetSucceeded(operation, publicationports.Result{
			Snapshot:      decision.Success.Snapshot,
			CleanupHandle: decision.Success.CleanupHandle,
		}); err != nil {
			return err
		}
		mutated = true
	}
	if decision.PersistRunning {
		if err := SetRunning(operation, decision.RunningWorker); err != nil {
			return err
		}
		mutated = true
	}
	if decision.PersistUpload {
		if err := SetUploadReady(operation, *decision.UploadStatus); err != nil {
			return err
		}
		mutated = true
	}
	if !mutated {
		return nil
	}

	return r.client.Update(ctx, operation)
}
