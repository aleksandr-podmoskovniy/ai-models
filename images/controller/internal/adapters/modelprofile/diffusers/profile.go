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

package diffusers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	profilecommon "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/common"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type Input struct {
	ModelDir           string
	Task               string
	SourceDeclaredTask string
	TaskHint           string
}

type SummaryInput struct {
	ModelIndexPayload      []byte
	WeightBytes            int64
	LargestWeightFileBytes int64
	WeightFileCount        int64
	Task                   string
	SourceDeclaredTask     string
	TaskHint               string
}

func Resolve(input Input) (publicationdata.ResolvedProfile, error) {
	if strings.TrimSpace(input.ModelDir) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("diffusers model directory must not be empty")
	}
	modelIndex, err := os.ReadFile(filepath.Join(input.ModelDir, "model_index.json"))
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}
	weights, err := totalWeightStats(input.ModelDir)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}
	return resolveSummary(modelIndex, weights, input.Task, input.SourceDeclaredTask, input.TaskHint)
}

func ResolveSummary(input SummaryInput) (publicationdata.ResolvedProfile, error) {
	if len(input.ModelIndexPayload) == 0 {
		return publicationdata.ResolvedProfile{}, errors.New("diffusers model_index payload must not be empty")
	}
	if input.WeightBytes <= 0 {
		return publicationdata.ResolvedProfile{}, errors.New("diffusers weight bytes must be positive")
	}
	return resolveSummary(input.ModelIndexPayload, weightStats{
		totalBytes:       input.WeightBytes,
		largestFileBytes: input.LargestWeightFileBytes,
		fileCount:        input.WeightFileCount,
	}, input.Task, input.SourceDeclaredTask, input.TaskHint)
}

func resolveSummary(modelIndexPayload []byte, weights weightStats, task, sourceDeclaredTask, taskHint string) (publicationdata.ResolvedProfile, error) {
	if weights.totalBytes <= 0 {
		return publicationdata.ResolvedProfile{}, errors.New("diffusers weight bytes must be positive")
	}
	modelIndex, err := decodeModelIndex(modelIndexPayload)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}
	pipelineClass := stringValue(modelIndex["_class_name"])
	family := inferFamilyFromPipelineClass(pipelineClass)
	resolvedTask, taskConfidence := resolveTask(task, sourceDeclaredTask, inferTaskFromPipelineClass(pipelineClass), taskHint)
	capabilities := profilecommon.Capabilities{}
	if taskConfidence.ReliablePublicFact() {
		capabilities = profilecommon.ResolveCapabilities(resolvedTask)
	}

	return publicationdata.ResolvedProfile{
		Task:           resolvedTask,
		TaskConfidence: taskConfidence,
		SourceCapabilities: publicationdata.SourceCapabilities{
			Tasks: profilecommon.DeclaredSourceTasks(sourceDeclaredTask),
		},
		Family:                 family,
		FamilyConfidence:       confidenceIfSet(family, publicationdata.ProfileConfidenceDerived),
		Architecture:           pipelineClass,
		ArchitectureConfidence: confidenceIfSet(pipelineClass, publicationdata.ProfileConfidenceExact),
		Format:                 string(modelsv1alpha1.ModelInputFormatDiffusers),
		SupportedEndpointTypes: capabilities.EndpointTypes,
		SupportedFeatures:      capabilities.Features,
		Footprint: publicationdata.ProfileFootprint{
			WeightsBytes:           weights.totalBytes,
			LargestWeightFileBytes: weights.largestFileBytes,
			ShardCount:             weights.fileCount,
			EstimatedWorkingSetGiB: profilecommon.EstimatedWorkingSetGiB(weights.totalBytes, 0, "", ""),
		},
	}, nil
}

func resolveTask(explicitTask, sourceDeclaredTask, derivedTask, taskHint string) (string, publicationdata.ProfileConfidence) {
	if task := strings.TrimSpace(explicitTask); task != "" {
		return task, publicationdata.ProfileConfidenceExact
	}
	if task := strings.TrimSpace(sourceDeclaredTask); task != "" {
		return task, publicationdata.ProfileConfidenceDeclared
	}
	if task := strings.TrimSpace(derivedTask); task != "" {
		return task, publicationdata.ProfileConfidenceDerived
	}
	if task := strings.TrimSpace(taskHint); task != "" {
		return task, publicationdata.ProfileConfidenceHint
	}
	return "", ""
}

func inferTaskFromPipelineClass(className string) string {
	return firstMatchingPipelineValue(normalizeClassName(className), pipelineTaskRules)
}

func inferFamilyFromPipelineClass(className string) string {
	return firstMatchingPipelineValue(normalizeClassName(className), pipelineFamilyRules)
}

type pipelineInferenceRule struct {
	value  string
	tokens []string
}

var pipelineTaskRules = []pipelineInferenceRule{
	{value: "image-to-video", tokens: []string{"imagetovideo", "image2video", "stablevideodiffusion"}},
	{value: "text-to-video", tokens: []string{"texttovideo", "text2video"}},
	{value: "video-to-video", tokens: []string{"videotovideo", "video2video"}},
	{value: "image-to-image", tokens: []string{"img2img", "imagetoimage"}},
	{value: "inpainting", tokens: []string{"inpaint"}},
	{value: "text-to-image", tokens: []string{
		"texttoimage",
		"text2image",
		"stablediffusionpipeline",
		"stablediffusionxlpipeline",
		"fluxpipeline",
		"pixart",
		"kandinsky",
	}},
	{value: "text-to-audio", tokens: []string{"audioldm", "audiogen"}},
}

var pipelineFamilyRules = []pipelineInferenceRule{
	{value: "stable-diffusion", tokens: []string{"stablediffusion"}},
	{value: "flux", tokens: []string{"flux"}},
	{value: "cogvideo", tokens: []string{"cogvideo"}},
	{value: "hunyuan-video", tokens: []string{"hunyuan"}},
	{value: "wan", tokens: []string{"wan"}},
	{value: "audioldm", tokens: []string{"audioldm"}},
}

func firstMatchingPipelineValue(className string, rules []pipelineInferenceRule) string {
	for _, rule := range rules {
		if containsAny(className, rule.tokens) {
			return rule.value
		}
	}
	return ""
}

func containsAny(value string, tokens []string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func decodeModelIndex(payload []byte) (map[string]any, error) {
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

type weightStats struct {
	totalBytes       int64
	largestFileBytes int64
	fileCount        int64
}

func totalWeightStats(root string) (weightStats, error) {
	var stats weightStats
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !modelformat.IsDiffusersWeightFile(relative) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		stats.add(info.Size())
		return nil
	})
	return stats, err
}

func (s *weightStats) add(size int64) {
	if size <= 0 {
		return
	}
	s.totalBytes += size
	if size > s.largestFileBytes {
		s.largestFileBytes = size
	}
	s.fileCount++
}

func stringValue(value any) string {
	typed, _ := value.(string)
	return strings.TrimSpace(typed)
}

func confidenceIfSet(value string, confidence publicationdata.ProfileConfidence) publicationdata.ProfileConfidence {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return confidence
}

func normalizeClassName(value string) string {
	replacer := strings.NewReplacer("-", "", "_", "", " ", "", ".", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(value)))
}
