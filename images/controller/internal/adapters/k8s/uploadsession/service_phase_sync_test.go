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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceSyncsControllerOwnedSessionPhases(t *testing.T) {
	t.Parallel()

	scheme := testkit.NewScheme(t)
	owner := testkit.NewUploadModel()
	owner.UID = types.UID("1111-2222")

	for _, tc := range []struct {
		name          string
		mark          func(context.Context, *Service, types.UID) error
		wantPhase     uploadsessionstate.Phase
		wantMessage   string
		wantHandleSet bool
	}{
		{
			name: "publishing",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkPublishing(ctx, ownerUID)
			},
			wantPhase:     uploadsessionstate.PhasePublishing,
			wantHandleSet: true,
		},
		{
			name: "completed",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkCompleted(ctx, ownerUID)
			},
			wantPhase: uploadsessionstate.PhaseCompleted,
		},
		{
			name: "failed",
			mark: func(ctx context.Context, service *Service, ownerUID types.UID) error {
				return service.MarkFailed(ctx, ownerUID, "publish failed")
			},
			wantPhase:     uploadsessionstate.PhaseFailed,
			wantMessage:   "publish failed",
			wantHandleSet: true,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			secret := mustUploadSessionSecret(t, owner.UID)
			handle := cleanuphandle.Handle{
				Kind: cleanuphandle.KindUploadStaging,
				UploadStaging: &cleanuphandle.UploadStagingHandle{
					Bucket:    "ai-models",
					Key:       "raw/1111-2222/model.gguf",
					FileName:  "model.gguf",
					SizeBytes: 128,
				},
			}
			if err := uploadsessionstate.MarkUploadedSecret(secret, handle); err != nil {
				t.Fatalf("MarkUploadedSecret() error = %v", err)
			}

			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(owner.DeepCopy(), secret).
				Build()

			service, err := NewService(kubeClient, scheme, testUploadOptions())
			if err != nil {
				t.Fatalf("NewService() error = %v", err)
			}

			if err := tc.mark(context.Background(), service, owner.UID); err != nil {
				t.Fatalf("phase sync error = %v", err)
			}

			updatedSecret := &corev1.Secret{}
			if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(secret), updatedSecret); err != nil {
				t.Fatalf("Get(updated secret) error = %v", err)
			}
			session, err := uploadsessionstate.SessionFromSecret(updatedSecret)
			if err != nil {
				t.Fatalf("SessionFromSecret() error = %v", err)
			}
			if session.Phase != tc.wantPhase {
				t.Fatalf("unexpected session phase %q", session.Phase)
			}
			if session.FailureMessage != tc.wantMessage {
				t.Fatalf("unexpected failure message %q", session.FailureMessage)
			}
			if got := session.StagedHandle != nil; got != tc.wantHandleSet {
				t.Fatalf("unexpected staged handle presence %v", got)
			}
		})
	}
}
