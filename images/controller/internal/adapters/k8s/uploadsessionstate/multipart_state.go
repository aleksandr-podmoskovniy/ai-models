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
	"strings"

	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	corev1 "k8s.io/api/core/v1"
)

const stateUploadedPartsKey = "multipartUploadedParts"

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
	for _, part := range state.UploadedParts {
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
	if err := setUploadedParts(secret, state.UploadedParts); err != nil {
		return err
	}
	return nil
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
	for _, part := range parts {
		switch {
		case part.PartNumber <= 0:
			return nil, errors.New("upload session uploaded part number must be positive")
		case strings.TrimSpace(part.ETag) == "":
			return nil, errors.New("upload session uploaded part ETag must not be empty")
		case part.SizeBytes < 0:
			return nil, errors.New("upload session uploaded part size must not be negative")
		}
	}
	return parts, nil
}

func clearMultipartState(secret *corev1.Secret) {
	delete(secret.Data, stateUploadIDKey)
	delete(secret.Data, stateObjectKey)
	delete(secret.Data, stateFileNameKey)
	delete(secret.Data, stateUploadedPartsKey)
}
