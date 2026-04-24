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

package publishstate

import (
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsTerminalOperationPhase(t *testing.T) {
	t.Parallel()

	cases := []struct {
		phase OperationPhase
		want  bool
	}{
		{phase: OperationPhasePending, want: false},
		{phase: OperationPhaseRunning, want: false},
		{phase: OperationPhaseStaged, want: false},
		{phase: OperationPhaseFailed, want: true},
		{phase: OperationPhaseSucceeded, want: true},
	}

	for _, tc := range cases {
		if got := IsTerminalOperationPhase(tc.phase); got != tc.want {
			t.Fatalf("IsTerminalOperationPhase(%q) = %v, want %v", tc.phase, got, tc.want)
		}
	}
}

func testUploadTokenSecretRef(name string) *modelsv1alpha1.UploadTokenSecretReference {
	return &modelsv1alpha1.UploadTokenSecretReference{
		Namespace: "team-a",
		Name:      name,
		Key:       "authorizationHeaderValue",
	}
}

func TestSameUploadStatus(t *testing.T) {
	t.Parallel()

	now := metav1.NewTime(time.Unix(1712345678, 0).UTC())
	later := metav1.NewTime(time.Unix(1712345688, 0).UTC())

	cases := []struct {
		name    string
		current *modelsv1alpha1.ModelUploadStatus
		desired *modelsv1alpha1.ModelUploadStatus
		want    bool
	}{
		{name: "both nil", want: true},
		{
			name: "equal",
			current: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			desired: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			want: true,
		},
		{
			name: "external URL differs",
			current: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/a",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			desired: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/b",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			want: false,
		},
		{
			name: "token secret ref differs",
			current: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			desired: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-b"),
				ExpiresAt:      &now,
			},
			want: false,
		},
		{
			name: "expiry differs",
			current: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &now,
			},
			desired: &modelsv1alpha1.ModelUploadStatus{
				ExternalURL:    "https://ai-models.example.com/upload/token",
				InClusterURL:   "http://upload-a.d8-ai-models.svc:8444/upload/token",
				Repository:     "registry.example/upload",
				TokenSecretRef: testUploadTokenSecretRef("upload-token-a"),
				ExpiresAt:      &later,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SameUploadStatus(tc.current, tc.desired); got != tc.want {
				t.Fatalf("SameUploadStatus() = %v, want %v", got, tc.want)
			}
		})
	}
}
