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
	"strconv"
	"strings"

	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	corev1 "k8s.io/api/core/v1"
)

func SaveProbeSecret(secret *corev1.Secret, expectedSizeBytes int64, state uploadsessionruntime.ProbeState) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	fileName := strings.TrimSpace(state.FileName)
	if fileName == "" {
		return errors.New("upload session probe file name must not be empty")
	}
	if expectedSizeBytes < 0 {
		return errors.New("upload session expected size bytes must not be negative")
	}

	ensureData(secret)
	secret.Data[stateProbeFileNameKey] = []byte(fileName)
	setOptionalInt64(secret, expectedSizeBytesKey, expectedSizeBytes)
	setOptionalString(secret, stateProbeFormatKey, string(state.ResolvedInputFormat))
	setRuntimePhase(secret, PhaseProbing, false)
	return nil
}

func ClearMultipartSecret(secret *corev1.Secret) error {
	if secret == nil {
		return errors.New("upload session secret must not be nil")
	}
	ensureData(secret)
	clearMultipartState(secret)
	if strings.TrimSpace(string(secret.Data[stateProbeFileNameKey])) != "" {
		setPhase(secret, PhaseProbing)
	} else {
		setPhase(secret, PhaseIssued)
	}
	delete(secret.Data, failureMessageKey)
	delete(secret.Data, stagedHandleKey)
	return nil
}

func MarkFailedSecret(secret *corev1.Secret, message string) error {
	return markTerminalSecret(secret, PhaseFailed, message, true)
}

func MarkAbortedSecret(secret *corev1.Secret, message string) error {
	return markTerminalSecret(secret, PhaseAborted, message, true)
}

func setRuntimePhase(secret *corev1.Secret, phase Phase, clearStagedHandle bool) {
	ensureData(secret)
	setPhase(secret, phase)
	delete(secret.Data, failureMessageKey)
	if clearStagedHandle {
		delete(secret.Data, stagedHandleKey)
	}
}

func setPhase(secret *corev1.Secret, phase Phase) {
	secret.Data[phaseKey] = []byte(string(phase))
}

func setOptionalString(secret *corev1.Secret, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		delete(secret.Data, key)
		return
	}
	secret.Data[key] = []byte(value)
}

func setOptionalInt64(secret *corev1.Secret, key string, value int64) {
	if value <= 0 {
		delete(secret.Data, key)
		return
	}
	secret.Data[key] = []byte(strconv.FormatInt(value, 10))
}
