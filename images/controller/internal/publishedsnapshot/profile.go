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

package publishedsnapshot

type ProfileConfidence string

const (
	ProfileConfidenceExact     ProfileConfidence = "Exact"
	ProfileConfidenceDerived   ProfileConfidence = "Derived"
	ProfileConfidenceEstimated ProfileConfidence = "Estimated"
	ProfileConfidenceHint      ProfileConfidence = "Hint"
)

type ProfileFootprint struct {
	WeightsBytes           int64
	LargestWeightFileBytes int64
	ShardCount             int64
	EstimatedWorkingSetGiB int64
}

func (c ProfileConfidence) ReliablePublicFact() bool {
	return c == ProfileConfidenceExact || c == ProfileConfidenceDerived
}

func (p ResolvedProfile) HasPartialConfidence() bool {
	return (p.Task != "" && lowConfidence(p.TaskConfidence)) ||
		(p.Family != "" && lowConfidence(p.FamilyConfidence)) ||
		(p.Architecture != "" && lowConfidence(p.ArchitectureConfidence)) ||
		(p.ParameterCount > 0 && lowConfidence(p.ParameterCountConfidence)) ||
		(p.Quantization != "" && lowConfidence(p.QuantizationConfidence)) ||
		(p.ContextWindowTokens > 0 && lowConfidence(p.ContextWindowTokensConfidence))
}

func (p ResolvedProfile) HasPublicSummary() bool {
	return p.Format != "" ||
		(p.Architecture != "" && p.ArchitectureConfidence.ReliablePublicFact()) ||
		(p.Task != "" && p.TaskConfidence.ReliablePublicFact()) ||
		(p.Family != "" && p.FamilyConfidence.ReliablePublicFact()) ||
		(p.ParameterCount > 0 && p.ParameterCountConfidence.ReliablePublicFact()) ||
		(p.Quantization != "" && p.QuantizationConfidence.ReliablePublicFact()) ||
		(p.ContextWindowTokens > 0 && p.ContextWindowTokensConfidence.ReliablePublicFact())
}

func lowConfidence(confidence ProfileConfidence) bool {
	return confidence == ProfileConfidenceEstimated || confidence == ProfileConfidenceHint
}
