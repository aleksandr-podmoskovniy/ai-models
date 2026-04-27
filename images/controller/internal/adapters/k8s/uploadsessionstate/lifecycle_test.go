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
	"testing"

	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
)

func TestSecretLifecycleMutatorsKeepProbeAndClearRuntimeState(t *testing.T) {
	t.Parallel()

	secret := mustSessionSecret(t)
	if err := SaveProbeSecret(secret, 256, uploadsessionruntime.ProbeState{
		FileName:            " model.gguf ",
		ResolvedInputFormat: " GGUF ",
	}); err != nil {
		t.Fatalf("SaveProbeSecret() error = %v", err)
	}
	if err := SaveMultipartSecret(secret, uploadsessionruntime.SessionState{
		UploadID: "upload-1",
		Key:      "raw/1111-2222/model.gguf",
		FileName: "model.gguf",
	}); err != nil {
		t.Fatalf("SaveMultipartSecret() error = %v", err)
	}
	if err := ClearMultipartSecret(secret); err != nil {
		t.Fatalf("ClearMultipartSecret() error = %v", err)
	}

	session, err := SessionFromSecret(secret)
	if err != nil {
		t.Fatalf("SessionFromSecret() error = %v", err)
	}
	if session.Phase != PhaseProbing || session.Multipart != nil || session.Probe == nil {
		t.Fatalf("unexpected session after clear %#v", session)
	}
}

func TestSecretLifecycleMutatorsPreserveStagedHandleContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mutate     func(*corev1.Secret) error
		wantHandle bool
	}{
		{name: "publishing-preserves-handle", mutate: MarkPublishingSecret, wantHandle: true},
		{
			name: "multipart-clears-handle",
			mutate: func(secret *corev1.Secret) error {
				return SaveMultipartSecret(secret, uploadsessionruntime.SessionState{
					UploadID: "upload-1",
					Key:      "raw/1111-2222/model.gguf",
					FileName: "model.gguf",
				})
			},
		},
		{name: "clear-multipart-clears-handle", mutate: ClearMultipartSecret},
		{name: "completed-clears-handle", mutate: MarkCompletedSecret},
		{
			name: "failed-clears-handle",
			mutate: func(secret *corev1.Secret) error {
				return MarkFailedSecret(secret, "upload failed")
			},
		},
		{
			name: "aborted-clears-handle",
			mutate: func(secret *corev1.Secret) error {
				return MarkAbortedSecret(secret, "upload aborted")
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			secret := mustUploadedSessionSecret(t)
			if err := test.mutate(secret); err != nil {
				t.Fatalf("mutate() error = %v", err)
			}
			_, hasHandle := secret.Data[stagedHandleKey]
			if hasHandle != test.wantHandle {
				t.Fatalf("staged handle present=%v, want %v", hasHandle, test.wantHandle)
			}
		})
	}
}

func TestSecretLifecycleTerminalNoopsPreserveExistingState(t *testing.T) {
	t.Parallel()

	for _, phase := range []Phase{PhaseCompleted, PhaseAborted, PhaseExpired} {
		phase := phase
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			secret := mustUploadedSessionSecret(t)
			secret.Data[phaseKey] = []byte(string(phase))
			before := string(secret.Data[stagedHandleKey])

			if err := MarkPublishingFailedSecret(secret, "publishing failed"); err != nil {
				t.Fatalf("MarkPublishingFailedSecret() error = %v", err)
			}
			if got := string(secret.Data[phaseKey]); got != string(phase) {
				t.Fatalf("phase = %q, want %q", got, phase)
			}
			if got := string(secret.Data[stagedHandleKey]); got != before {
				t.Fatalf("staged handle changed")
			}
		})
	}
}

func mustUploadedSessionSecret(t *testing.T) *corev1.Secret {
	t.Helper()
	secret := mustSessionSecret(t)
	if err := MarkUploadedSecret(secret, cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/1111-2222/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 128,
		},
	}); err != nil {
		t.Fatalf("MarkUploadedSecret() error = %v", err)
	}
	return secret
}
