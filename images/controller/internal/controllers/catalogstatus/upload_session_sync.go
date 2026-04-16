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

package catalogstatus

import (
	"context"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationplan "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type uploadSessionPhaseSync struct {
	ApplyOnCleanupRequeue bool
	Apply                 func(context.Context) error
}

func (r *baseReconciler) planUploadSessionPhaseSync(
	mode publicationplan.ExecutionMode,
	ownerUID types.UID,
	sourceType modelsv1alpha1.ModelSourceType,
	observation publicationdomain.Observation,
	desired modelsv1alpha1.ModelStatus,
) uploadSessionPhaseSync {
	if r == nil || r.uploadSessions == nil {
		return uploadSessionPhaseSync{}
	}
	if sourceType != modelsv1alpha1.ModelSourceTypeUpload || strings.TrimSpace(string(ownerUID)) == "" {
		return uploadSessionPhaseSync{}
	}

	switch mode {
	case publicationplan.ExecutionModeUpload:
		if observation.Phase == publicationdomain.OperationPhaseStaged {
			return uploadSessionPhaseSync{
				ApplyOnCleanupRequeue: true,
				Apply: func(ctx context.Context) error {
					return r.uploadSessions.MarkPublishing(ctx, ownerUID)
				},
			}
		}
	case publicationplan.ExecutionModeSourceWorker:
		switch desired.Phase {
		case modelsv1alpha1.ModelPhasePublishing:
			return uploadSessionPhaseSync{
				Apply: func(ctx context.Context) error {
					return r.uploadSessions.MarkPublishing(ctx, ownerUID)
				},
			}
		case modelsv1alpha1.ModelPhaseReady:
			return uploadSessionPhaseSync{
				Apply: func(ctx context.Context) error {
					return r.uploadSessions.MarkCompleted(ctx, ownerUID)
				},
			}
		case modelsv1alpha1.ModelPhaseFailed:
			message := uploadSessionFailureMessage(observation, desired)
			return uploadSessionPhaseSync{
				Apply: func(ctx context.Context) error {
					return r.uploadSessions.MarkFailed(ctx, ownerUID, message)
				},
			}
		}
	}

	return uploadSessionPhaseSync{}
}

func runUploadSessionPhaseSync(
	ctx context.Context,
	sync uploadSessionPhaseSync,
	onCleanupRequeue bool,
) error {
	if sync.Apply == nil || sync.ApplyOnCleanupRequeue != onCleanupRequeue {
		return nil
	}
	return sync.Apply(ctx)
}

func uploadSessionFailureMessage(
	observation publicationdomain.Observation,
	desired modelsv1alpha1.ModelStatus,
) string {
	if message := strings.TrimSpace(observation.Message); message != "" {
		return message
	}
	for _, condition := range desired.Conditions {
		if condition.Status != metav1.ConditionFalse {
			continue
		}
		switch modelsv1alpha1.ModelConditionType(condition.Type) {
		case modelsv1alpha1.ModelConditionReady,
			modelsv1alpha1.ModelConditionValidated,
			modelsv1alpha1.ModelConditionArtifactResolved:
			if message := strings.TrimSpace(condition.Message); message != "" {
				return message
			}
		}
	}
	return "publication failed"
}

func planFailedUploadSessionPhaseSync(
	r *baseReconciler,
	mode publicationplan.ExecutionMode,
	ownerUID types.UID,
	sourceType modelsv1alpha1.ModelSourceType,
	desired modelsv1alpha1.ModelStatus,
	message string,
) uploadSessionPhaseSync {
	if r == nil {
		return uploadSessionPhaseSync{}
	}
	return r.planUploadSessionPhaseSync(mode, ownerUID, sourceType, publicationdomain.Observation{
		Phase:   publicationdomain.OperationPhaseFailed,
		Message: message,
	}, desired)
}
