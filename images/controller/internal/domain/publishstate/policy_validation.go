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
	"fmt"
	"slices"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type policyValidationResult struct {
	Valid   bool
	Reason  modelsv1alpha1.ModelConditionReason
	Message string
}

func validatePublishedModel(spec modelsv1alpha1.ModelSpec, snapshot publicationdata.Snapshot) policyValidationResult {
	inferredType := inferModelType(snapshot.Resolved.Task)
	supportedEndpoints := inferEndpointTypes(snapshot.Resolved.Task)

	if result := validateModelType(spec.ModelType, inferredType); !result.Valid {
		return result
	}

	if result := validateUsagePolicy(spec.UsagePolicy, supportedEndpoints); !result.Valid {
		return result
	}

	if result := validateLaunchPolicy(spec.LaunchPolicy, snapshot.Resolved); !result.Valid {
		return result
	}

	if result := validateOptimizationPolicy(spec.Optimization, inferredType, supportedEndpoints); !result.Valid {
		return result
	}

	return policyValidationResult{
		Valid:   true,
		Reason:  modelsv1alpha1.ModelConditionReasonValidationSucceeded,
		Message: "controller validated the published model record successfully",
	}
}

func validateModelType(expected, inferred modelsv1alpha1.ModelType) policyValidationResult {
	if expected == "" {
		return validPolicy()
	}
	switch {
	case inferred == "":
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonModelTypeMismatch,
			"controller could not infer a model type from the resolved model profile",
		)
	case inferred != expected:
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonModelTypeMismatch,
			fmt.Sprintf("spec.modelType=%q does not match resolved model type %q", expected, inferred),
		)
	default:
		return validPolicy()
	}
}

func validateUsagePolicy(policy *modelsv1alpha1.ModelUsagePolicy, supportedEndpoints []string) policyValidationResult {
	if policy == nil || len(policy.AllowedEndpointTypes) == 0 {
		return validPolicy()
	}
	allowed := normalizeEndpointTypes(policy.AllowedEndpointTypes)
	if len(intersectStrings(allowed, supportedEndpoints)) > 0 {
		return validPolicy()
	}
	return invalidPolicy(
		modelsv1alpha1.ModelConditionReasonEndpointTypeNotSupported,
		fmt.Sprintf("usagePolicy.allowedEndpointTypes=%v does not intersect with resolved supported endpoint types %v", policy.AllowedEndpointTypes, supportedEndpoints),
	)
}

func validateLaunchPolicy(policy *modelsv1alpha1.ModelLaunchPolicy, resolved publicationdata.ResolvedProfile) policyValidationResult {
	if policy == nil {
		return validPolicy()
	}
	if result := validatePreferredRuntime(policy, resolved.CompatibleRuntimes); !result.Valid {
		return result
	}
	if len(policy.AllowedRuntimes) > 0 && len(intersectRuntimeEngines(policy.AllowedRuntimes, resolved.CompatibleRuntimes)) == 0 {
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonRuntimeNotSupported,
			fmt.Sprintf("launchPolicy.allowedRuntimes=%v does not intersect with resolved compatible runtimes %v", policy.AllowedRuntimes, resolved.CompatibleRuntimes),
		)
	}
	if len(policy.AllowedAcceleratorVendors) > 0 && len(intersectAcceleratorVendors(policy.AllowedAcceleratorVendors, resolved.CompatibleAcceleratorVendors)) == 0 {
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonAcceleratorPolicyConflict,
			fmt.Sprintf("launchPolicy.allowedAcceleratorVendors=%v does not intersect with resolved compatible accelerator vendors %v", policy.AllowedAcceleratorVendors, resolved.CompatibleAcceleratorVendors),
		)
	}
	if len(policy.AllowedPrecisions) > 0 && len(intersectPrecisions(policy.AllowedPrecisions, resolved.CompatiblePrecisions)) == 0 {
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonAcceleratorPolicyConflict,
			fmt.Sprintf("launchPolicy.allowedPrecisions=%v does not intersect with resolved compatible precisions %v", policy.AllowedPrecisions, resolved.CompatiblePrecisions),
		)
	}
	return validPolicy()
}

func validatePreferredRuntime(policy *modelsv1alpha1.ModelLaunchPolicy, compatibleRuntimes []string) policyValidationResult {
	if policy == nil || policy.PreferredRuntime == "" {
		return validPolicy()
	}
	if len(policy.AllowedRuntimes) > 0 && !slices.Contains(policy.AllowedRuntimes, policy.PreferredRuntime) {
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonRuntimeNotSupported,
			fmt.Sprintf("launchPolicy.preferredRuntime=%q must be included in launchPolicy.allowedRuntimes", policy.PreferredRuntime),
		)
	}
	if containsString(compatibleRuntimes, string(policy.PreferredRuntime)) {
		return validPolicy()
	}
	return invalidPolicy(
		modelsv1alpha1.ModelConditionReasonRuntimeNotSupported,
		fmt.Sprintf("launchPolicy.preferredRuntime=%q is not compatible with the resolved model profile", policy.PreferredRuntime),
	)
}

func validateOptimizationPolicy(
	policy *modelsv1alpha1.ModelOptimizationPolicy,
	inferredType modelsv1alpha1.ModelType,
	supportedEndpoints []string,
) policyValidationResult {
	if len(draftModelRefs(policy)) == 0 {
		return validPolicy()
	}
	if inferredType != modelsv1alpha1.ModelTypeLLM {
		return invalidPolicy(
			modelsv1alpha1.ModelConditionReasonOptimizationNotSupported,
			"optimization.speculativeDecoding is only supported for LLM models",
		)
	}
	if containsString(supportedEndpoints, string(modelsv1alpha1.ModelEndpointTypeChat)) ||
		containsString(supportedEndpoints, string(modelsv1alpha1.ModelEndpointTypeTextGeneration)) {
		return validPolicy()
	}
	return invalidPolicy(
		modelsv1alpha1.ModelConditionReasonOptimizationNotSupported,
		"optimization.speculativeDecoding requires a model that supports chat or text generation",
	)
}

func validPolicy() policyValidationResult {
	return policyValidationResult{Valid: true}
}

func invalidPolicy(reason modelsv1alpha1.ModelConditionReason, message string) policyValidationResult {
	return policyValidationResult{
		Valid:   false,
		Reason:  reason,
		Message: strings.TrimSpace(message),
	}
}

func inferModelType(task string) modelsv1alpha1.ModelType {
	switch normalizeValue(task) {
	case "text-generation", "text2text-generation", "summarization", "conversational":
		return modelsv1alpha1.ModelTypeLLM
	case "feature-extraction", "embeddings", "text-embeddings-inference", "sentence-similarity":
		return modelsv1alpha1.ModelTypeEmbeddings
	case "rerank", "reranker", "text-ranking":
		return modelsv1alpha1.ModelTypeReranker
	case "automatic-speech-recognition", "speech-to-text":
		return modelsv1alpha1.ModelTypeSpeechToText
	case "translation":
		return modelsv1alpha1.ModelTypeTranslation
	default:
		return ""
	}
}

func inferEndpointTypes(task string) []string {
	switch inferModelType(task) {
	case modelsv1alpha1.ModelTypeLLM:
		return []string{
			string(modelsv1alpha1.ModelEndpointTypeChat),
			string(modelsv1alpha1.ModelEndpointTypeTextGeneration),
		}
	case modelsv1alpha1.ModelTypeEmbeddings:
		return []string{string(modelsv1alpha1.ModelEndpointTypeEmbeddings)}
	case modelsv1alpha1.ModelTypeReranker:
		return []string{string(modelsv1alpha1.ModelEndpointTypeRerank)}
	case modelsv1alpha1.ModelTypeSpeechToText:
		return []string{string(modelsv1alpha1.ModelEndpointTypeSpeechToText)}
	case modelsv1alpha1.ModelTypeTranslation:
		return []string{string(modelsv1alpha1.ModelEndpointTypeTranslation)}
	default:
		return nil
	}
}

func draftModelRefs(policy *modelsv1alpha1.ModelOptimizationPolicy) []modelsv1alpha1.ModelReference {
	if policy == nil || policy.SpeculativeDecoding == nil {
		return nil
	}
	return policy.SpeculativeDecoding.DraftModelRefs
}

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
