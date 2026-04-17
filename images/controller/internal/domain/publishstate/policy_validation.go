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

package publishstate

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type policyValidationResult struct {
	Valid   bool
	Reason  modelsv1alpha1.ModelConditionReason
	Message string
}

func validatePublishedModel(spec modelsv1alpha1.ModelSpec, snapshot publicationdata.Snapshot) policyValidationResult {
	_ = spec
	_ = snapshot
	return policyValidationResult{
		Valid:   true,
		Reason:  modelsv1alpha1.ModelConditionReasonValidationSucceeded,
		Message: "controller accepted the calculated published model metadata",
	}
}
