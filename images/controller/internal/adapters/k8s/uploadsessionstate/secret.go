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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ExpiresAtAnnotationKey = "ai.deckhouse.io/upload-expires-at"
	OwnerGenerationKey     = "ai.deckhouse.io/upload-owner-generation"

	tokenHashKey           = "tokenHash"
	expectedSizeBytesKey   = "expectedSizeBytes"
	stagingKeyPrefixKey    = "stagingKeyPrefix"
	declaredInputFormatKey = "declaredInputFormat"
	phaseKey               = "phase"
	failureMessageKey      = "failureMessage"
	stagedHandleKey        = "stagedHandle"

	stateUploadIDKey      = "multipartUploadID"
	stateObjectKey        = "multipartKey"
	stateFileNameKey      = "multipartFileName"
	stateProbeFileNameKey = "probeFileName"
	stateProbeFormatKey   = "probeResolvedInputFormat"
)

type Phase string

const (
	PhaseIssued     Phase = "issued"
	PhaseProbing    Phase = "probing"
	PhaseUploading  Phase = "uploading"
	PhaseUploaded   Phase = "uploaded"
	PhasePublishing Phase = "publishing"
	PhaseCompleted  Phase = "completed"
	PhaseFailed     Phase = "failed"
	PhaseAborted    Phase = "aborted"
	PhaseExpired    Phase = "expired"
)

type Session struct {
	Name                string
	UploadTokenHash     string
	ExpectedSizeBytes   int64
	StagingKeyPrefix    string
	DeclaredInputFormat modelsv1alpha1.ModelInputFormat
	OwnerUID            string
	OwnerKind           string
	OwnerName           string
	OwnerNamespace      string
	OwnerGeneration     int64
	ExpiresAt           metav1.Time
	Phase               Phase
	Probe               *uploadsessionruntime.ProbeState
	Multipart           *uploadsessionruntime.SessionState
	FailureMessage      string
	StagedHandle        *cleanuphandle.Handle
}

type SessionSpec struct {
	Name                string
	Namespace           string
	Token               string
	ExpectedSizeBytes   int64
	StagingKeyPrefix    string
	DeclaredInputFormat modelsv1alpha1.ModelInputFormat
	OwnerGeneration     int64
	ExpiresAt           time.Time
}

func NewSecret(spec SessionSpec) (*corev1.Secret, error) {
	switch {
	case strings.TrimSpace(spec.Name) == "":
		return nil, errors.New("upload session secret name must not be empty")
	case strings.TrimSpace(spec.Namespace) == "":
		return nil, errors.New("upload session secret namespace must not be empty")
	case strings.TrimSpace(spec.Token) == "":
		return nil, errors.New("upload session token must not be empty")
	case strings.TrimSpace(spec.StagingKeyPrefix) == "":
		return nil, errors.New("upload session staging key prefix must not be empty")
	case spec.ExpectedSizeBytes < 0:
		return nil, errors.New("upload session expected size bytes must not be negative")
	case spec.ExpiresAt.IsZero():
		return nil, errors.New("upload session expiry must not be zero")
	}
	if _, err := parseInputFormat([]byte(spec.DeclaredInputFormat)); err != nil {
		return nil, err
	}

	data := map[string][]byte{
		stagingKeyPrefixKey: []byte(strings.TrimSpace(spec.StagingKeyPrefix)),
		phaseKey:            []byte(string(PhaseIssued)),
	}
	if err := setToken(secretDataAccessor{data: data}, spec.Token); err != nil {
		return nil, err
	}
	if spec.ExpectedSizeBytes > 0 {
		data[expectedSizeBytesKey] = []byte(strconv.FormatInt(spec.ExpectedSizeBytes, 10))
	}
	if strings.TrimSpace(string(spec.DeclaredInputFormat)) != "" {
		data[declaredInputFormatKey] = []byte(strings.TrimSpace(string(spec.DeclaredInputFormat)))
	}

	annotations := map[string]string{
		ExpiresAtAnnotationKey: spec.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if spec.OwnerGeneration > 0 {
		annotations[OwnerGenerationKey] = strconv.FormatInt(spec.OwnerGeneration, 10)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        strings.TrimSpace(spec.Name),
			Namespace:   strings.TrimSpace(spec.Namespace),
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}, nil
}

func SessionFromSecret(secret *corev1.Secret) (*Session, error) {
	if secret == nil {
		return nil, errors.New("upload session secret must not be nil")
	}
	if len(secret.Data) == 0 {
		return nil, errors.New("upload session secret data must not be empty")
	}

	tokenHash, err := tokenHashFromSecret(secret)
	if err != nil {
		return nil, err
	}
	stagingKeyPrefix := strings.TrimSpace(string(secret.Data[stagingKeyPrefixKey]))
	if stagingKeyPrefix == "" {
		return nil, errors.New("upload session staging key prefix must not be empty")
	}

	expectedSizeBytes, err := parseExpectedSizeBytes(secret.Data[expectedSizeBytesKey])
	if err != nil {
		return nil, err
	}
	declaredInputFormat, err := parseInputFormat(secret.Data[declaredInputFormatKey])
	if err != nil {
		return nil, err
	}
	expiresAt, err := ExpiresAtFromSecret(secret)
	if err != nil {
		return nil, err
	}
	ownerGeneration, err := ownerGenerationFromSecret(secret)
	if err != nil {
		return nil, err
	}
	phase, err := parsePhase(secret.Data[phaseKey])
	if err != nil {
		return nil, err
	}
	probe, err := probeStateFromSecret(secret)
	if err != nil {
		return nil, err
	}
	multipart, err := multipartStateFromSecret(secret)
	if err != nil {
		return nil, err
	}

	session := &Session{
		Name:                secret.Name,
		UploadTokenHash:     tokenHash,
		ExpectedSizeBytes:   expectedSizeBytes,
		StagingKeyPrefix:    stagingKeyPrefix,
		DeclaredInputFormat: declaredInputFormat,
		OwnerUID:            strings.TrimSpace(secret.Labels[resourcenames.OwnerUIDLabelKey]),
		OwnerKind:           strings.TrimSpace(secret.Annotations[resourcenames.OwnerKindAnnotationKey]),
		OwnerName:           strings.TrimSpace(secret.Annotations[resourcenames.OwnerNameAnnotationKey]),
		OwnerNamespace:      strings.TrimSpace(secret.Annotations[resourcenames.OwnerNamespaceAnnotationKey]),
		OwnerGeneration:     ownerGeneration,
		ExpiresAt:           expiresAt,
		Phase:               phase,
		Probe:               probe,
		Multipart:           multipart,
		FailureMessage:      strings.TrimSpace(string(secret.Data[failureMessageKey])),
	}
	if rawHandle := strings.TrimSpace(string(secret.Data[stagedHandleKey])); rawHandle != "" {
		handle, err := cleanuphandle.Decode(rawHandle)
		if err != nil {
			return nil, fmt.Errorf("decode upload staged handle: %w", err)
		}
		session.StagedHandle = &handle
	}

	return session, nil
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
