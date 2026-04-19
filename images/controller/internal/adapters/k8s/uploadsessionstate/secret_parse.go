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

package uploadsessionstate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	corev1 "k8s.io/api/core/v1"
)

func parseExpectedSizeBytes(raw []byte) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, nil
	}
	sizeBytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse upload session expected size bytes: %w", err)
	}
	if sizeBytes < 0 {
		return 0, errors.New("upload session expected size bytes must not be negative")
	}
	return sizeBytes, nil
}

func parseInputFormat(raw []byte) (modelsv1alpha1.ModelInputFormat, error) {
	value := modelsv1alpha1.ModelInputFormat(strings.TrimSpace(string(raw)))
	switch value {
	case "", modelsv1alpha1.ModelInputFormatSafetensors, modelsv1alpha1.ModelInputFormatGGUF:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported upload session input format %q", value)
	}
}

func parsePhase(raw []byte) (Phase, error) {
	switch phase := Phase(strings.TrimSpace(string(raw))); phase {
	case "", PhaseIssued:
		return PhaseIssued, nil
	case PhaseProbing, PhaseUploading, PhaseUploaded, PhasePublishing, PhaseCompleted, PhaseFailed, PhaseAborted, PhaseExpired:
		return phase, nil
	default:
		return "", fmt.Errorf("unsupported upload session phase %q", phase)
	}
}

func probeStateFromSecret(secret *corev1.Secret) (*uploadsessionruntime.ProbeState, error) {
	fileName := strings.TrimSpace(string(secret.Data[stateProbeFileNameKey]))
	if fileName == "" {
		return nil, nil
	}
	resolvedInputFormat, err := parseInputFormat(secret.Data[stateProbeFormatKey])
	if err != nil {
		return nil, err
	}
	return &uploadsessionruntime.ProbeState{
		FileName:            fileName,
		ResolvedInputFormat: resolvedInputFormat,
	}, nil
}

func ensureData(secret *corev1.Secret) {
	if secret.Data == nil {
		secret.Data = make(map[string][]byte, 10)
	}
}

func ownerGenerationFromSecret(secret *corev1.Secret) (int64, error) {
	raw := strings.TrimSpace(secret.Annotations[OwnerGenerationKey])
	if raw == "" {
		return 0, nil
	}
	generation, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse upload session owner generation: %w", err)
	}
	if generation < 0 {
		return 0, errors.New("upload session owner generation must not be negative")
	}
	return generation, nil
}
