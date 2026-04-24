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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const stateUploadedPartsKey = "multipartUploadedParts"

var ErrTokenHashMissing = errors.New("upload session token hash must not be empty")

func SetToken(secret *corev1.Secret, rawToken string) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	ensureData(secret)
	return storeTokenHash(secret.Data, rawToken)
}

func ExpiresAtFromSecret(secret *corev1.Secret) (metav1.Time, error) {
	if secret == nil {
		return metav1.Time{}, errors.New("upload session secret must not be nil")
	}
	raw := strings.TrimSpace(secret.Annotations[ExpiresAtAnnotationKey])
	if raw == "" {
		return metav1.Time{}, errors.New("upload session expiry annotation is missing")
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return metav1.Time{}, fmt.Errorf("parse upload session expiry: %w", err)
	}
	return metav1.NewTime(value.UTC()), nil
}

func parseExpectedSizeBytes(raw []byte) (int64, error) {
	return parseOptionalNonNegativeInt64(raw, "upload session expected size bytes")
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
	if secret == nil {
		return 0, errors.New("upload session secret must not be nil")
	}
	return parseOptionalNonNegativeInt64([]byte(secret.Annotations[OwnerGenerationKey]), "upload session owner generation")
}

func multipartStateFromSecret(secret *corev1.Secret) (*uploadsessionruntime.SessionState, error) {
	uploadID := strings.TrimSpace(string(secret.Data[stateUploadIDKey]))
	key := strings.TrimSpace(string(secret.Data[stateObjectKey]))
	fileName := strings.TrimSpace(string(secret.Data[stateFileNameKey]))
	uploadedParts, err := uploadedPartsFromSecret(secret.Data[stateUploadedPartsKey])
	if err != nil {
		return nil, err
	}
	if uploadID == "" && key == "" && fileName == "" && len(uploadedParts) == 0 {
		return nil, nil
	}
	state := uploadsessionruntime.SessionState{
		UploadID:      uploadID,
		Key:           key,
		FileName:      fileName,
		UploadedParts: uploadedParts,
	}
	if err := validateMultipartState(state); err != nil {
		return nil, err
	}
	return &state, nil
}

func validateMultipartState(state uploadsessionruntime.SessionState) error {
	switch {
	case strings.TrimSpace(state.UploadID) == "":
		return errors.New("upload session state upload id must not be empty")
	case strings.TrimSpace(state.Key) == "":
		return errors.New("upload session state key must not be empty")
	case strings.TrimSpace(state.FileName) == "":
		return errors.New("upload session state file name must not be empty")
	}
	return validateUploadedParts(state.UploadedParts)
}

func validateUploadedParts(parts []uploadsessionruntime.UploadedPart) error {
	for _, part := range parts {
		switch {
		case part.PartNumber <= 0:
			return errors.New("upload session uploaded part number must be positive")
		case strings.TrimSpace(part.ETag) == "":
			return errors.New("upload session uploaded part ETag must not be empty")
		case part.SizeBytes < 0:
			return errors.New("upload session uploaded part size must not be negative")
		}
	}
	return nil
}

func setMultipartState(secret *corev1.Secret, state uploadsessionruntime.SessionState) error {
	if err := validateMultipartState(state); err != nil {
		return err
	}
	ensureData(secret)
	secret.Data[stateUploadIDKey] = []byte(strings.TrimSpace(state.UploadID))
	secret.Data[stateObjectKey] = []byte(strings.TrimSpace(state.Key))
	secret.Data[stateFileNameKey] = []byte(strings.TrimSpace(state.FileName))
	return setUploadedParts(secret, state.UploadedParts)
}

func setUploadedParts(secret *corev1.Secret, parts []uploadsessionruntime.UploadedPart) error {
	ensureData(secret)
	if len(parts) == 0 {
		delete(secret.Data, stateUploadedPartsKey)
		return nil
	}
	payload, err := json.Marshal(parts)
	if err != nil {
		return fmt.Errorf("marshal upload session multipart manifest: %w", err)
	}
	secret.Data[stateUploadedPartsKey] = payload
	return nil
}

func uploadedPartsFromSecret(raw []byte) ([]uploadsessionruntime.UploadedPart, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return nil, nil
	}
	var parts []uploadsessionruntime.UploadedPart
	if err := json.Unmarshal([]byte(value), &parts); err != nil {
		return nil, fmt.Errorf("unmarshal upload session multipart manifest: %w", err)
	}
	if err := validateUploadedParts(parts); err != nil {
		return nil, err
	}
	return parts, nil
}

func clearMultipartState(secret *corev1.Secret) {
	delete(secret.Data, stateUploadIDKey)
	delete(secret.Data, stateObjectKey)
	delete(secret.Data, stateFileNameKey)
	delete(secret.Data, stateUploadedPartsKey)
}

func tokenHashFromSecret(secret *corev1.Secret) (string, error) {
	tokenHash := strings.TrimSpace(string(secret.Data[tokenHashKey]))
	if tokenHash == "" {
		return "", ErrTokenHashMissing
	}
	if !isValidTokenHash(tokenHash) {
		return "", fmt.Errorf("upload session token hash must be a 64-character lowercase hex string")
	}
	return tokenHash, nil
}

func storeTokenHash(data map[string][]byte, rawToken string) error {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return errors.New("upload session token must not be empty")
	}
	data[tokenHashKey] = []byte(uploadsessiontoken.Hash(rawToken))
	return nil
}

func isValidTokenHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, ch := range value {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}

func parseOptionalNonNegativeInt64(raw []byte, field string) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", field, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must not be negative", field)
	}
	return parsed, nil
}
