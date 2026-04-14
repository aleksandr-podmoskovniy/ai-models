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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func normalizeEndpointTypes(values []modelsv1alpha1.ModelEndpointType) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(string(value)); normalized != "" {
			result = append(result, normalized)
		}
	}
	return result
}

func intersectRuntimeEngines(allowed []modelsv1alpha1.ModelRuntimeEngine, resolved []string) []string {
	normalizedAllowed := make([]string, 0, len(allowed))
	for _, value := range allowed {
		if value != "" {
			normalizedAllowed = append(normalizedAllowed, string(value))
		}
	}
	return intersectStrings(normalizedAllowed, resolved)
}

func intersectAcceleratorVendors(allowed []modelsv1alpha1.ModelAcceleratorVendor, resolved []string) []string {
	normalizedAllowed := make([]string, 0, len(allowed))
	for _, value := range allowed {
		if value != "" {
			normalizedAllowed = append(normalizedAllowed, string(value))
		}
	}
	return intersectStrings(normalizedAllowed, resolved)
}

func intersectPrecisions(allowed []modelsv1alpha1.ModelPrecision, resolved []string) []string {
	normalizedAllowed := make([]string, 0, len(allowed))
	for _, value := range allowed {
		switch value {
		case modelsv1alpha1.ModelPrecisionFP32:
			normalizedAllowed = append(normalizedAllowed, "fp32")
		case modelsv1alpha1.ModelPrecisionFP16:
			normalizedAllowed = append(normalizedAllowed, "fp16", "f16")
		case modelsv1alpha1.ModelPrecisionBF16:
			normalizedAllowed = append(normalizedAllowed, "bf16")
		case modelsv1alpha1.ModelPrecisionFP8:
			normalizedAllowed = append(normalizedAllowed, "fp8")
		case modelsv1alpha1.ModelPrecisionINT8:
			normalizedAllowed = append(normalizedAllowed, "int8")
		case modelsv1alpha1.ModelPrecisionINT4:
			normalizedAllowed = append(normalizedAllowed, "int4")
		}
	}
	return intersectStrings(normalizedAllowed, resolved)
}

func intersectStrings(allowed, resolved []string) []string {
	result := make([]string, 0, len(allowed))
	seen := map[string]struct{}{}
	for _, raw := range allowed {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	for _, raw := range resolved {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(expected) {
			return true
		}
	}
	return false
}

func normalizeValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
