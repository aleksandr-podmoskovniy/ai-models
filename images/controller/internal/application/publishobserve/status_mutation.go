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

package publishobserve

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type CatalogStatusMutationInput struct {
	Current modelsv1alpha1.ModelStatus
	Runtime CatalogStatusRuntimeResult
}

type CatalogStatusRuntimeResult struct {
	Generation    int64
	SourceType    modelsv1alpha1.ModelSourceType
	Observation   publicationdomain.Observation
	DeleteRuntime bool
}

type CatalogStatusMutationPlan struct {
	Status                     modelsv1alpha1.ModelStatus
	CleanupHandle              *cleanuphandle.Handle
	Requeue                    bool
	DeleteRuntime              bool
	DeleteRuntimeBeforePersist bool
}

func PlanCatalogStatusMutation(input CatalogStatusMutationInput) (CatalogStatusMutationPlan, error) {
	projection, err := publicationdomain.ProjectStatus(
		input.Current,
		input.Runtime.Generation,
		input.Runtime.SourceType,
		input.Runtime.Observation,
	)
	if err != nil {
		return CatalogStatusMutationPlan{}, err
	}

	return CatalogStatusMutationPlan{
		Status:                     projection.Status,
		CleanupHandle:              projection.CleanupHandle,
		Requeue:                    projection.Requeue,
		DeleteRuntime:              input.Runtime.DeleteRuntime,
		DeleteRuntimeBeforePersist: input.Runtime.DeleteRuntime && input.Runtime.Observation.Phase == publicationdomain.OperationPhaseFailed,
	}, nil
}

func PlanFailedCatalogStatusMutation(
	current modelsv1alpha1.ModelStatus,
	generation int64,
	sourceType modelsv1alpha1.ModelSourceType,
	message string,
) (CatalogStatusMutationPlan, error) {
	return PlanCatalogStatusMutation(CatalogStatusMutationInput{
		Current: current,
		Runtime: CatalogStatusRuntimeResult{
			Generation:  generation,
			SourceType:  sourceType,
			Observation: failedObservation(message),
		},
	})
}
