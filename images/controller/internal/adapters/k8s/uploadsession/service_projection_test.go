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

package uploadsession

import (
	"context"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceGetOrCreateProjectsUploadedAndFailedSessionState(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		mutate         func(t *testing.T, secret *corev1.Secret)
		wantPhase      corev1.PodPhase
		wantTermSubstr string
	}{
		{
			name: "uploaded",
			mutate: func(t *testing.T, secret *corev1.Secret) {
				t.Helper()
				handle := cleanuphandle.Handle{
					Kind: cleanuphandle.KindUploadStaging,
					UploadStaging: &cleanuphandle.UploadStagingHandle{
						Bucket:    "ai-models",
						Key:       "raw/1111-2222/model.gguf",
						FileName:  "model.gguf",
						SizeBytes: 128,
					},
				}
				encoded, err := cleanuphandle.Encode(handle)
				if err != nil {
					t.Fatalf("Encode() error = %v", err)
				}
				secret.Data["phase"] = []byte(string(uploadsessionstate.PhaseUploaded))
				secret.Data["stagedHandle"] = []byte(encoded)
			},
			wantPhase:      corev1.PodSucceeded,
			wantTermSubstr: "\"kind\":\"UploadStaging\"",
		},
		{
			name: "failed",
			mutate: func(t *testing.T, secret *corev1.Secret) {
				t.Helper()
				secret.Data["phase"] = []byte(string(uploadsessionstate.PhaseFailed))
				secret.Data["failureMessage"] = []byte("upload failed")
			},
			wantPhase:      corev1.PodFailed,
			wantTermSubstr: "upload failed",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scheme := testkit.NewScheme(t)
			owner := testkit.NewUploadModel()
			owner.UID = types.UID("1111-2222")

			secret := mustUploadSessionSecret(t, owner.UID)
			tc.mutate(t, secret)

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(owner, secret).
				Build()

			service, err := NewService(kubeClient, scheme, testUploadOptions())
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}
			request := testUploadRequest()
			request.Owner.UID = owner.UID

			handle, created, err := service.GetOrCreate(context.Background(), owner, request)
			if err != nil {
				t.Fatalf("GetOrCreate() error = %v", err)
			}
			if created {
				t.Fatal("expected existing session to be reused")
			}
			if handle == nil || handle.Phase != tc.wantPhase {
				t.Fatalf("unexpected handle %#v", handle)
			}
			if !strings.Contains(handle.TerminationMessage, tc.wantTermSubstr) {
				t.Fatalf("unexpected termination message %q", handle.TerminationMessage)
			}
		})
	}
}
