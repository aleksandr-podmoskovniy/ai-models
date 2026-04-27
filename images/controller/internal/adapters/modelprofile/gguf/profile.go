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

package gguf

import (
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	profilecommon "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/common"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type Input struct {
	ModelDir string
	Task     string
}

type SummaryInput struct {
	ModelFileName  string
	ModelSizeBytes int64
	Task           string
}

var (
	quantizationPattern = regexp.MustCompile(`(?i)\b(i?q[2-8](?:_[a-z0-9]+)*)\b`)
	sizePattern         = regexp.MustCompile(`(?i)\b(\d+(?:\.\d+)?)b\b`)
)

func Resolve(input Input) (publicationdata.ResolvedProfile, error) {
	if strings.TrimSpace(input.ModelDir) == "" {
		return publicationdata.ResolvedProfile{}, errors.New("gguf model directory must not be empty")
	}

	modelPath, modelSizeBytes, err := firstGGUFFile(input.ModelDir)
	if err != nil {
		return publicationdata.ResolvedProfile{}, err
	}

	stem := strings.TrimSuffix(filepath.Base(modelPath), filepath.Ext(modelPath))
	return resolveFromSummary(stem, modelSizeBytes, input.Task), nil
}

func ResolveSummary(input SummaryInput) (publicationdata.ResolvedProfile, error) {
	modelFileName := strings.TrimSpace(input.ModelFileName)
	if modelFileName == "" {
		return publicationdata.ResolvedProfile{}, errors.New("gguf model file name must not be empty")
	}
	if input.ModelSizeBytes <= 0 {
		return publicationdata.ResolvedProfile{}, errors.New("gguf model size must be positive")
	}

	stem := strings.TrimSuffix(filepath.Base(modelFileName), filepath.Ext(modelFileName))
	return resolveFromSummary(stem, input.ModelSizeBytes, input.Task), nil
}

func resolveFromSummary(stem string, modelSizeBytes int64, task string) publicationdata.ResolvedProfile {
	quantization := detectQuantization(stem)
	precision := detectPrecision(quantization)
	parameterCount := detectParameterCount(stem)
	parameterConfidence := publicationdata.ProfileConfidenceHint
	if parameterCount <= 0 {
		parameterCount = profilecommon.EstimateParameterCountFromBytes(modelSizeBytes, precision, quantization)
		parameterConfidence = publicationdata.ProfileConfidenceEstimated
	}
	endpoints := []string(nil)
	task = strings.TrimSpace(task)
	taskConfidence := publicationdata.ProfileConfidence("")
	if task != "" {
		taskConfidence = publicationdata.ProfileConfidenceExact
		endpoints = profilecommon.EndpointTypes(task)
	}

	return publicationdata.ResolvedProfile{
		Task:                     task,
		TaskConfidence:           taskConfidence,
		Family:                   normalizeFamily(stem),
		FamilyConfidence:         publicationdata.ProfileConfidenceHint,
		Format:                   "GGUF",
		ParameterCount:           parameterCount,
		ParameterCountConfidence: parameterConfidence,
		Quantization:             quantization,
		QuantizationConfidence:   confidenceIfSet(quantization, publicationdata.ProfileConfidenceHint),
		SupportedEndpointTypes:   endpoints,
		Footprint: publicationdata.ProfileFootprint{
			WeightsBytes:           modelSizeBytes,
			LargestWeightFileBytes: modelSizeBytes,
			ShardCount:             1,
			EstimatedWorkingSetGiB: profilecommon.EstimatedWorkingSetGiB(modelSizeBytes, parameterCount, precision, quantization),
		},
	}
}

func firstGGUFFile(root string) (string, int64, error) {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", 0, err
	}
	if !rootInfo.IsDir() {
		looksLikeGGUF, err := hasGGUFMagic(root)
		if err != nil {
			return "", 0, err
		}
		if !strings.HasSuffix(strings.ToLower(rootInfo.Name()), ".gguf") && !looksLikeGGUF {
			return "", 0, errors.New("gguf model file was not found")
		}
		return root, rootInfo.Size(), nil
	}

	var match string
	var matchSize int64

	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".gguf") {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			match = path
			matchSize = info.Size()
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return "", 0, err
	}
	if strings.TrimSpace(match) == "" {
		return "", 0, errors.New("gguf model file was not found")
	}
	return match, matchSize, nil
}

func hasGGUFMagic(path string) (bool, error) {
	stream, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer stream.Close()

	header := make([]byte, 4)
	n, err := io.ReadFull(stream, header)
	switch {
	case err == nil:
		return n == 4 && string(header) == "GGUF", nil
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return false, nil
	default:
		return false, err
	}
}

func detectQuantization(name string) string {
	match := quantizationPattern.FindStringSubmatch(strings.ToLower(name))
	if len(match) > 1 {
		return match[1]
	}
	if strings.Contains(strings.ToLower(name), "f16") {
		return "f16"
	}
	if strings.Contains(strings.ToLower(name), "bf16") {
		return "bf16"
	}
	return ""
}

func detectPrecision(quantization string) string {
	switch profilecommon.BytesPerParameter("", quantization) {
	case 0.5:
		return "int4"
	case 1:
		return "int8"
	}

	switch strings.ToLower(strings.TrimSpace(quantization)) {
	case "f16":
		return "fp16"
	case "bf16":
		return "bf16"
	default:
		return ""
	}
}

func detectParameterCount(name string) int64 {
	matches := sizePattern.FindAllStringSubmatch(strings.ToLower(name), -1)
	var maxBillions float64
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(match[1])
		if value == "" {
			continue
		}

		number, err := parseBillions(value)
		if err != nil {
			continue
		}
		if number > maxBillions {
			maxBillions = number
		}
	}
	if maxBillions <= 0 {
		return 0
	}
	return int64(math.Round(maxBillions * 1_000_000_000))
}

func parseBillions(raw string) (float64, error) {
	return strconv.ParseFloat(raw, 64)
}

func normalizeFamily(name string) string {
	normalized := strings.TrimSpace(name)
	if match := quantizationPattern.FindStringIndex(strings.ToLower(normalized)); match != nil {
		normalized = strings.Trim(normalized[:match[0]], "-_ .")
	}
	normalized = strings.Trim(sizePattern.ReplaceAllString(normalized, ""), "-_ .")
	if normalized == "" {
		return strings.TrimSpace(name)
	}
	return normalized
}

func confidenceIfSet(value string, confidence publicationdata.ProfileConfidence) publicationdata.ProfileConfidence {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return confidence
}
