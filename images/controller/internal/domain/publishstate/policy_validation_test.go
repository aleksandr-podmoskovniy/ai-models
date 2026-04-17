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
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestValidatePublishedModelAcceptsCalculatedMetadata(t *testing.T) {
	t.Parallel()

	result := validatePublishedModel(modelsv1alpha1.ModelSpec{}, publicationdata.Snapshot{
		Resolved: publicationdata.ResolvedProfile{
			Task:                         "text-generation",
			SupportedEndpointTypes:       []string{"Chat", "TextGeneration"},
			CompatibleRuntimes:           []string{"VLLM"},
			CompatibleAcceleratorVendors: []string{"NVIDIA"},
			CompatiblePrecisions:         []string{"int4"},
		},
	})
	if !result.Valid || result.Reason != modelsv1alpha1.ModelConditionReasonValidationSucceeded {
		t.Fatalf("unexpected result %#v", result)
	}
}
