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

package publicationops

import (
	"encoding/json"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRequestFromConfigMapFailsClosed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   *corev1.ConfigMap
		wantErr bool
	}{
		{
			name:    "nil configmap",
			wantErr: true,
		},
		{
			name:    "missing request payload",
			input:   &corev1.ConfigMap{},
			wantErr: true,
		},
		{
			name: "whitespace-only request payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{requestDataKey: "   "},
			},
			wantErr: true,
		},
		{
			name: "malformed request payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{requestDataKey: "{not-json"},
			},
			wantErr: true,
		},
		{
			name: "invalid request payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{requestDataKey: `{"owner":{"kind":"Model","name":"deepseek-r1"}}`},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := RequestFromConfigMap(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestResultFromConfigMapFailsClosed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   *corev1.ConfigMap
		wantErr bool
	}{
		{
			name:    "nil configmap",
			wantErr: true,
		},
		{
			name:    "missing result payload",
			input:   &corev1.ConfigMap{},
			wantErr: true,
		},
		{
			name: "whitespace-only result payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{resultDataKey: "   "},
			},
			wantErr: true,
		},
		{
			name: "malformed result payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{resultDataKey: "{not-json"},
			},
			wantErr: true,
		},
		{
			name: "invalid result payload",
			input: &corev1.ConfigMap{
				Data: map[string]string{resultDataKey: `{"snapshot":{},"cleanupHandle":{"kind":"BackendArtifact","backend":{}}}`},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ResultFromConfigMap(tc.input)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestCodecHelpersHandleNilAndWhitespace(t *testing.T) {
	t.Parallel()

	if IsManagedConfigMap(nil) {
		t.Fatal("nil configmap must not be managed")
	}
	if WorkerResultFromConfigMap(nil) != "" {
		t.Fatal("expected empty worker result for nil configmap")
	}
	if WorkerFailureFromConfigMap(nil) != "" {
		t.Fatal("expected empty worker failure for nil configmap")
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{managedLabelKey: managedLabelValue},
		},
		Data: map[string]string{
			workerResultDataKey:  "  result  ",
			workerFailureDataKey: "  failed  ",
			uploadDataKey:        "{not-json}",
		},
	}
	if !IsManagedConfigMap(configMap) {
		t.Fatal("expected managed configmap")
	}
	if got, want := WorkerResultFromConfigMap(configMap), "result"; got != want {
		t.Fatalf("unexpected worker result %q", got)
	}
	if got, want := WorkerFailureFromConfigMap(configMap), "failed"; got != want {
		t.Fatalf("unexpected worker failure %q", got)
	}
	if _, err := UploadStatusFromConfigMap(configMap); err == nil {
		t.Fatal("expected invalid upload payload to fail")
	}
}

func TestStatusFromConfigMapDefaultsPending(t *testing.T) {
	t.Parallel()

	status := StatusFromConfigMap(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				messageAnnotationKey: "  waiting  ",
				workerAnnotationKey:  "  worker-a  ",
			},
		},
	})
	if got, want := status.Phase, PhasePending; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := status.Message, "waiting"; got != want {
		t.Fatalf("unexpected message %q", got)
	}
	if got, want := status.WorkerName, "worker-a"; got != want {
		t.Fatalf("unexpected worker %q", got)
	}
}

func TestUploadStatusFromConfigMapReturnsNilWhenMissing(t *testing.T) {
	t.Parallel()

	status, err := UploadStatusFromConfigMap(&corev1.ConfigMap{})
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status, got %#v", status)
	}

	status, err = UploadStatusFromConfigMap(&corev1.ConfigMap{
		Data: map[string]string{uploadDataKey: "   "},
	})
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if status != nil {
		t.Fatalf("expected nil status for whitespace payload, got %#v", status)
	}

	status, err = UploadStatusFromConfigMap(&corev1.ConfigMap{
		Data: map[string]string{uploadDataKey: `{"command":"curl","repository":"repo","expiresAt":"2026-04-03T10:00:00Z"}`},
	})
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if status == nil || status.Command != "curl" || status.Repository != "repo" || status.ExpiresAt == nil {
		t.Fatalf("unexpected upload status %#v", status)
	}

	if _, err := UploadStatusFromConfigMap(&corev1.ConfigMap{
		Data: map[string]string{uploadDataKey: `{}`},
	}); err == nil {
		t.Fatal("expected semantically invalid upload payload to fail")
	}

	if _, err := UploadStatusFromConfigMap(&corev1.ConfigMap{
		Data: map[string]string{uploadDataKey: `{"command":"curl","expiresAt":"2026-04-03T10:00:00Z"}`},
	}); err == nil {
		t.Fatal("expected upload payload without repository to fail")
	}
}

func TestValidatePersistedStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   *corev1.ConfigMap
		status  publicationports.Status
		wantErr bool
	}{
		{
			name:    "pending is accepted",
			input:   &corev1.ConfigMap{},
			status:  publicationports.Status{Phase: publicationports.PhasePending},
			wantErr: false,
		},
		{
			name: "running with malformed upload fails",
			input: &corev1.ConfigMap{
				Data: map[string]string{uploadDataKey: `{}`},
			},
			status:  publicationports.Status{Phase: publicationports.PhaseRunning},
			wantErr: true,
		},
		{
			name:    "succeeded without result fails",
			input:   &corev1.ConfigMap{},
			status:  publicationports.Status{Phase: publicationports.PhaseSucceeded},
			wantErr: true,
		},
		{
			name: "succeeded with valid result passes",
			input: &corev1.ConfigMap{
				Data: map[string]string{resultDataKey: mustMarshalResult(t)},
			},
			status:  publicationports.Status{Phase: publicationports.PhaseSucceeded},
			wantErr: false,
		},
		{
			name:    "unsupported phase fails",
			input:   &corev1.ConfigMap{},
			status:  publicationports.Status{Phase: publicationports.Phase("Weird")},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validatePersistedStatus(tc.input, tc.status)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validatePersistedStatus() error = %v", err)
			}
		})
	}
}

func TestSetRunningFailsClosed(t *testing.T) {
	t.Parallel()

	if err := SetRunning(nil, "worker-a"); err == nil {
		t.Fatal("expected nil configmap error")
	}
	if err := SetRunning(&corev1.ConfigMap{}, " "); err == nil {
		t.Fatal("expected empty worker error")
	}
}

func TestSetUploadReadyFailsClosed(t *testing.T) {
	t.Parallel()

	if err := SetUploadReady(nil, modelsv1alpha1.ModelUploadStatus{}); err == nil {
		t.Fatal("expected nil configmap error")
	}
	if err := SetUploadReady(&corev1.ConfigMap{}, modelsv1alpha1.ModelUploadStatus{}); err == nil {
		t.Fatal("expected invalid upload status error")
	}
}

func TestSetFailedClearsUploadAndTrimsMessage(t *testing.T) {
	t.Parallel()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{workerAnnotationKey: "worker-a"},
		},
		Data: map[string]string{
			uploadDataKey: `{"command":"curl"}`,
			resultDataKey: `{"stale":"result"}`,
		},
	}
	if err := SetFailed(configMap, "  failed import  "); err != nil {
		t.Fatalf("SetFailed() error = %v", err)
	}
	if got, want := configMap.Annotations[phaseAnnotationKey], string(PhaseFailed); got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := configMap.Annotations[messageAnnotationKey], "failed import"; got != want {
		t.Fatalf("unexpected message %q", got)
	}
	if _, found := configMap.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared on failure")
	}
	if _, found := configMap.Data[resultDataKey]; found {
		t.Fatal("result payload must be cleared on failure")
	}
}

func TestSetSucceededClearsUploadAndPersistsResult(t *testing.T) {
	t.Parallel()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				messageAnnotationKey: "failed import",
			},
		},
		Data: map[string]string{
			uploadDataKey: `{"command":"curl"}`,
		},
	}

	result := publicationports.Result{
		Snapshot: publication.Snapshot{
			Identity: publication.Identity{
				Scope:     publication.ScopeNamespaced,
				Namespace: "team-a",
				Name:      "deepseek-r1",
			},
			Source: publication.SourceProvenance{
				Type: modelsv1alpha1.ModelSourceTypeHuggingFace,
			},
			Artifact: publication.PublishedArtifact{
				Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
				URI:  "registry.internal.local/ai-models/catalog/team-a/deepseek-r1@sha256:deadbeef",
			},
			Result: publication.Result{
				State: "Published",
				Ready: true,
			},
		},
		CleanupHandle: cleanuphandle.Handle{
			Kind: cleanuphandle.KindBackendArtifact,
			Backend: &cleanuphandle.BackendArtifactHandle{
				Reference: "registry.internal.local/ai-models/catalog/team-a/deepseek-r1@sha256:deadbeef",
			},
		},
	}

	if err := SetSucceeded(configMap, result); err != nil {
		t.Fatalf("SetSucceeded() error = %v", err)
	}
	if got, want := configMap.Annotations[phaseAnnotationKey], string(PhaseSucceeded); got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if _, found := configMap.Annotations[messageAnnotationKey]; found {
		t.Fatal("failure message must be cleared on success")
	}
	if _, found := configMap.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared on success")
	}
	if _, err := ResultFromConfigMap(configMap); err != nil {
		t.Fatalf("ResultFromConfigMap() error = %v", err)
	}
}

func TestSetUploadReadyPersistsPayload(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Unix(1712345678, 0).UTC())
	configMap := &corev1.ConfigMap{}
	if err := SetUploadReady(configMap, modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:  &expiresAt,
		Repository: "registry.internal.local/model:upload",
		Command:    "curl -T file",
	}); err != nil {
		t.Fatalf("SetUploadReady() error = %v", err)
	}
	upload, err := UploadStatusFromConfigMap(configMap)
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if upload == nil || upload.Command != "curl -T file" {
		t.Fatalf("unexpected upload payload %#v", upload)
	}
}

func mustMarshalResult(t *testing.T) string {
	t.Helper()

	payload, err := json.Marshal(sampleResult())
	if err != nil {
		t.Fatalf("json.Marshal(sampleResult()) error = %v", err)
	}
	return string(payload)
}
