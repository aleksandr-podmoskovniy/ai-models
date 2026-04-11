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
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
)

func MarkUploadedSecret(secret *corev1.Secret, handle cleanuphandle.Handle) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	if err := handle.Validate(); err != nil {
		return err
	}
	encoded, err := cleanuphandle.Encode(handle)
	if err != nil {
		return err
	}
	ensureData(secret)
	secret.Data[phaseKey] = []byte(string(PhaseUploaded))
	secret.Data[stagedHandleKey] = []byte(encoded)
	delete(secret.Data, failureMessageKey)
	return nil
}

func MarkPublishingSecret(secret *corev1.Secret) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	phase, err := phaseFromSecret(secret)
	if err != nil {
		return err
	}
	switch phase {
	case PhaseCompleted, PhaseFailed, PhaseAborted, PhaseExpired:
		return nil
	default:
	}
	ensureData(secret)
	secret.Data[phaseKey] = []byte(string(PhasePublishing))
	delete(secret.Data, failureMessageKey)
	return nil
}

func MarkCompletedSecret(secret *corev1.Secret) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	phase, err := phaseFromSecret(secret)
	if err != nil {
		return err
	}
	switch phase {
	case PhaseFailed, PhaseAborted, PhaseExpired:
		return nil
	default:
	}
	ensureData(secret)
	secret.Data[phaseKey] = []byte(string(PhaseCompleted))
	delete(secret.Data, failureMessageKey)
	delete(secret.Data, stagedHandleKey)
	return nil
}

func MarkPublishingFailedSecret(secret *corev1.Secret, message string) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("upload session terminal message must not be empty")
	}
	phase, err := phaseFromSecret(secret)
	if err != nil {
		return err
	}
	switch phase {
	case PhaseCompleted, PhaseAborted, PhaseExpired:
		return nil
	default:
	}
	ensureData(secret)
	secret.Data[phaseKey] = []byte(string(PhaseFailed))
	secret.Data[failureMessageKey] = []byte(message)
	return nil
}

func MarkExpiredSecret(secret *corev1.Secret, message string) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("upload session terminal message must not be empty")
	}
	ensureData(secret)
	delete(secret.Data, stagedHandleKey)
	secret.Data[phaseKey] = []byte(string(PhaseExpired))
	secret.Data[failureMessageKey] = []byte(message)
	return nil
}

func phaseFromSecret(secret *corev1.Secret) (Phase, error) {
	if secret == nil {
		return "", errors.New("upload session secret must not be nil")
	}
	return parsePhase(secret.Data[phaseKey])
}
