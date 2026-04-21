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

package directuploadstate

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	OwnerGenerationAnnotationKey = "ai.deckhouse.io/direct-upload-owner-generation"
	stateKey                     = "state.json"
)

type SecretSpec struct {
	Name            string
	Namespace       string
	OwnerGeneration int64
}

func NewSecret(spec SecretSpec) (*corev1.Secret, error) {
	switch {
	case strings.TrimSpace(spec.Name) == "":
		return nil, errors.New("direct upload state secret name must not be empty")
	case strings.TrimSpace(spec.Namespace) == "":
		return nil, errors.New("direct upload state secret namespace must not be empty")
	case spec.OwnerGeneration <= 0:
		return nil, errors.New("direct upload state owner generation must be greater than zero")
	}

	payload, err := marshalState(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseIdle,
		Stage: modelpackports.DirectUploadStateStageIdle,
	})
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.TrimSpace(spec.Name),
			Namespace: strings.TrimSpace(spec.Namespace),
			Annotations: map[string]string{
				OwnerGenerationAnnotationKey: strconv.FormatInt(spec.OwnerGeneration, 10),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			stateKey: payload,
		},
	}, nil
}

func StateFromSecret(secret *corev1.Secret) (modelpackports.DirectUploadState, error) {
	if secret == nil {
		return modelpackports.DirectUploadState{}, errors.New("direct upload state secret must not be nil")
	}
	payload := secret.Data[stateKey]
	if len(payload) == 0 {
		return modelpackports.DirectUploadState{}, errors.New("direct upload state payload is missing")
	}

	var state modelpackports.DirectUploadState
	if err := json.Unmarshal(payload, &state); err != nil {
		return modelpackports.DirectUploadState{}, fmt.Errorf("decode direct upload state: %w", err)
	}
	return normalizeState(state)
}

func OwnerGenerationFromSecret(secret *corev1.Secret) (int64, error) {
	if secret == nil {
		return 0, errors.New("direct upload state secret must not be nil")
	}
	raw := strings.TrimSpace(secret.Annotations[OwnerGenerationAnnotationKey])
	if raw == "" {
		return 0, errors.New("direct upload state owner generation annotation is missing")
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse direct upload state owner generation: %w", err)
	}
	if value <= 0 {
		return 0, errors.New("direct upload state owner generation must be greater than zero")
	}
	return value, nil
}

func marshalState(state modelpackports.DirectUploadState) ([]byte, error) {
	normalized, err := normalizeState(state)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode direct upload state: %w", err)
	}
	return payload, nil
}

func normalizeState(state modelpackports.DirectUploadState) (modelpackports.DirectUploadState, error) {
	if state.Phase == "" {
		state.Phase = modelpackports.DirectUploadStatePhaseIdle
	}
	switch state.Phase {
	case modelpackports.DirectUploadStatePhaseIdle,
		modelpackports.DirectUploadStatePhaseRunning,
		modelpackports.DirectUploadStatePhaseCompleted,
		modelpackports.DirectUploadStatePhaseFailed:
	default:
		return modelpackports.DirectUploadState{}, fmt.Errorf("unsupported direct upload state phase %q", state.Phase)
	}
	if state.Stage == "" {
		state.Stage = inferStage(state)
	}
	switch state.Stage {
	case modelpackports.DirectUploadStateStageIdle,
		modelpackports.DirectUploadStateStageStarting,
		modelpackports.DirectUploadStateStageUploading,
		modelpackports.DirectUploadStateStageResumed,
		modelpackports.DirectUploadStateStageSealing,
		modelpackports.DirectUploadStateStageCommitted:
	default:
		return modelpackports.DirectUploadState{}, fmt.Errorf("unsupported direct upload state stage %q", state.Stage)
	}

	state.FailureMessage = strings.TrimSpace(state.FailureMessage)
	if state.Phase != modelpackports.DirectUploadStatePhaseFailed {
		state.FailureMessage = ""
	}

	completed := make([]modelpackports.DirectUploadLayerDescriptor, 0, len(state.CompletedLayers))
	for _, layer := range state.CompletedLayers {
		normalizedLayer, err := normalizeCompletedLayer(layer)
		if err != nil {
			return modelpackports.DirectUploadState{}, err
		}
		completed = append(completed, normalizedLayer)
	}
	state.CompletedLayers = completed

	if state.CurrentLayer != nil {
		current, err := normalizeCurrentLayer(*state.CurrentLayer)
		if err != nil {
			return modelpackports.DirectUploadState{}, err
		}
		state.CurrentLayer = &current
	}
	if state.Phase != modelpackports.DirectUploadStatePhaseRunning {
		state.Stage = modelpackports.DirectUploadStateStageIdle
		state.CurrentLayer = nil
	}
	if state.Phase == modelpackports.DirectUploadStatePhaseRunning && state.Stage == modelpackports.DirectUploadStateStageIdle {
		state.Stage = inferStage(state)
	}

	return state, nil
}

func inferStage(state modelpackports.DirectUploadState) modelpackports.DirectUploadStateStage {
	if state.Phase != modelpackports.DirectUploadStatePhaseRunning {
		return modelpackports.DirectUploadStateStageIdle
	}
	switch {
	case state.CurrentLayer != nil:
		if state.CurrentLayer.UploadedSizeBytes > 0 {
			return modelpackports.DirectUploadStateStageUploading
		}
		return modelpackports.DirectUploadStateStageStarting
	case len(state.CompletedLayers) > 0:
		return modelpackports.DirectUploadStateStageCommitted
	default:
		return modelpackports.DirectUploadStateStageStarting
	}
}

func normalizeCompletedLayer(layer modelpackports.DirectUploadLayerDescriptor) (modelpackports.DirectUploadLayerDescriptor, error) {
	layer.Key = strings.TrimSpace(layer.Key)
	layer.Digest = strings.TrimSpace(layer.Digest)
	layer.DiffID = strings.TrimSpace(layer.DiffID)
	layer.MediaType = strings.TrimSpace(layer.MediaType)
	layer.TargetPath = strings.TrimSpace(layer.TargetPath)
	if layer.Key == "" {
		return modelpackports.DirectUploadLayerDescriptor{}, errors.New("completed direct upload layer key must not be empty")
	}
	if layer.Digest == "" {
		return modelpackports.DirectUploadLayerDescriptor{}, errors.New("completed direct upload layer digest must not be empty")
	}
	if layer.SizeBytes <= 0 {
		return modelpackports.DirectUploadLayerDescriptor{}, errors.New("completed direct upload layer size must be greater than zero")
	}
	if layer.MediaType == "" {
		return modelpackports.DirectUploadLayerDescriptor{}, errors.New("completed direct upload layer mediaType must not be empty")
	}
	if layer.TargetPath == "" {
		return modelpackports.DirectUploadLayerDescriptor{}, errors.New("completed direct upload layer targetPath must not be empty")
	}
	return layer, nil
}

func normalizeCurrentLayer(layer modelpackports.DirectUploadCurrentLayer) (modelpackports.DirectUploadCurrentLayer, error) {
	layer.Key = strings.TrimSpace(layer.Key)
	layer.SessionToken = strings.TrimSpace(layer.SessionToken)
	if layer.Key == "" {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer key must not be empty")
	}
	if layer.SessionToken == "" {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer session token must not be empty")
	}
	if layer.PartSizeBytes <= 0 {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer part size must be greater than zero")
	}
	if layer.TotalSizeBytes <= 0 {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer total size must be greater than zero")
	}
	if layer.UploadedSizeBytes < 0 {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer uploaded size must not be negative")
	}
	if layer.UploadedSizeBytes > layer.TotalSizeBytes {
		return modelpackports.DirectUploadCurrentLayer{}, errors.New("direct upload current layer uploaded size must not exceed total size")
	}
	return layer, nil
}
