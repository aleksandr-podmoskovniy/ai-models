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
	"reflect"
	"testing"
	"time"

	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestNewSecretBuildsIssuedSession(t *testing.T) {
	t.Parallel()

	expiresAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	secret, err := NewSecret(SessionSpec{
		Name:              "ai-model-upload-auth-1111-2222",
		Namespace:         "d8-ai-models",
		Token:             "upload-token",
		ExpectedSizeBytes: 128,
		StagingKeyPrefix:  "raw/1111-2222",
		ExpiresAt:         expiresAt,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	session, err := SessionFromSecret(secret)
	if err != nil {
		t.Fatalf("SessionFromSecret() error = %v", err)
	}
	if session.Phase != PhaseIssued {
		t.Fatalf("unexpected phase %q", session.Phase)
	}
	if session.UploadTokenHash != uploadsessiontoken.Hash("upload-token") || session.StagingKeyPrefix != "raw/1111-2222" {
		t.Fatalf("unexpected session %#v", session)
	}
	if _, found := secret.Data[tokenKey]; found {
		t.Fatalf("raw upload token must not be persisted in secret data: %#v", secret.Data)
	}
	if got := string(secret.Data[tokenHashKey]); got != uploadsessiontoken.Hash("upload-token") {
		t.Fatalf("unexpected token hash %q", got)
	}
	if !session.ExpiresAt.Equal(&metav1.Time{Time: expiresAt}) {
		t.Fatalf("unexpected expiry %s", session.ExpiresAt.Time)
	}
}

func TestMigrateLegacyTokenRewritesSecretData(t *testing.T) {
	t.Parallel()

	secret := mustSessionSecret(t)
	secret.Data[tokenKey] = []byte("legacy-token")
	delete(secret.Data, tokenHashKey)

	rawToken, changed, err := MigrateLegacyToken(secret)
	if err != nil {
		t.Fatalf("MigrateLegacyToken() error = %v", err)
	}
	if !changed || rawToken != "legacy-token" {
		t.Fatalf("unexpected migration result changed=%v rawToken=%q", changed, rawToken)
	}
	if _, found := secret.Data[tokenKey]; found {
		t.Fatalf("raw upload token must be removed after migration: %#v", secret.Data)
	}
	if got := string(secret.Data[tokenHashKey]); got != uploadsessiontoken.Hash("legacy-token") {
		t.Fatalf("unexpected token hash %q", got)
	}
}

func TestClientTracksMultipartAndTerminalState(t *testing.T) {
	t.Parallel()

	clientset := kubernetesfake.NewSimpleClientset(mustSessionSecret(t))
	client, err := New(clientset, "d8-ai-models")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	multipart := uploadsessionruntime.SessionState{
		UploadID: "upload-1",
		Key:      "raw/1111-2222/model.gguf",
		FileName: "model.gguf",
		UploadedParts: []uploadsessionruntime.UploadedPart{
			{PartNumber: 1, ETag: "etag-1", SizeBytes: 64},
		},
	}
	if err := client.SaveMultipart(t.Context(), "ai-model-upload-auth-1111-2222", multipart); err != nil {
		t.Fatalf("SaveMultipart() error = %v", err)
	}

	session, found, err := client.Load(t.Context(), "ai-model-upload-auth-1111-2222")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !found || session.Multipart == nil || !reflect.DeepEqual(*session.Multipart, multipart) {
		t.Fatalf("unexpected multipart session %#v found=%v", session, found)
	}
	if session.Phase != PhaseUploading {
		t.Fatalf("unexpected phase %q", session.Phase)
	}

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/1111-2222/model.gguf",
			FileName:  "model.gguf",
			SizeBytes: 128,
		},
	}
	if err := client.MarkUploaded(t.Context(), "ai-model-upload-auth-1111-2222", handle); err != nil {
		t.Fatalf("MarkUploaded() error = %v", err)
	}

	session, found, err = client.Load(t.Context(), "ai-model-upload-auth-1111-2222")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !found || session.Phase != PhaseUploaded || session.StagedHandle == nil {
		t.Fatalf("unexpected uploaded session %#v found=%v", session, found)
	}
	if session.Multipart == nil || len(session.Multipart.UploadedParts) != 1 {
		t.Fatalf("expected multipart manifest to be preserved, got %#v", session.Multipart)
	}
}

func TestClientMarksFailedAndAborted(t *testing.T) {
	t.Parallel()

	for _, phase := range []Phase{PhaseFailed, PhaseAborted, PhaseExpired} {
		phase := phase
		t.Run(string(phase), func(t *testing.T) {
			t.Parallel()

			clientset := kubernetesfake.NewSimpleClientset(mustSessionSecret(t))
			client, err := New(clientset, "d8-ai-models")
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			var markErr error
			switch phase {
			case PhaseFailed:
				markErr = client.MarkFailed(t.Context(), "ai-model-upload-auth-1111-2222", "upload failed")
			case PhaseAborted:
				markErr = client.MarkAborted(t.Context(), "ai-model-upload-auth-1111-2222", "upload aborted")
			case PhaseExpired:
				markErr = client.MarkExpired(t.Context(), "ai-model-upload-auth-1111-2222", "upload expired")
			}
			if markErr != nil {
				t.Fatalf("mark terminal state error = %v", markErr)
			}

			session, found, err := client.Load(t.Context(), "ai-model-upload-auth-1111-2222")
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !found || session.Phase != phase {
				t.Fatalf("unexpected session %#v found=%v", session, found)
			}
			if session.FailureMessage == "" {
				t.Fatal("expected terminal failure message")
			}
		})
	}
}

func TestClientSaveProbeMarksProbingPhase(t *testing.T) {
	t.Parallel()

	clientset := kubernetesfake.NewSimpleClientset(mustSessionSecret(t))
	client, err := New(clientset, "d8-ai-models")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := client.SaveProbe(t.Context(), "ai-model-upload-auth-1111-2222", uploadsessionruntime.ProbeState{
		FileName:            "model.gguf",
		ResolvedInputFormat: "GGUF",
	}); err != nil {
		t.Fatalf("SaveProbe() error = %v", err)
	}

	session, found, err := client.Load(t.Context(), "ai-model-upload-auth-1111-2222")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !found || session.Phase != PhaseProbing {
		t.Fatalf("unexpected session %#v found=%v", session, found)
	}
}

func mustSessionSecret(t *testing.T) *corev1.Secret {
	t.Helper()
	secret, err := NewSecret(SessionSpec{
		Name:              "ai-model-upload-auth-1111-2222",
		Namespace:         "d8-ai-models",
		Token:             "upload-token",
		ExpectedSizeBytes: 128,
		StagingKeyPrefix:  "raw/1111-2222",
		ExpiresAt:         time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}
	return secret
}
