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

package deletion

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFinalizeDeleteWaitsForRuntimeCleanupBeforeRemovingFinalizer(t *testing.T) {
	t.Parallel()

	got := FinalizeDelete(FinalizeDeleteInput{
		HasFinalizer:           true,
		RuntimeResourcePresent: true,
	})

	if !got.DeleteRuntimeResources || !got.UpdateStatus || !got.Requeue {
		t.Fatalf("unexpected decision %#v", got)
	}
	if got.EnsureGarbageCollectionRequest {
		t.Fatalf("did not expect garbage collection request without direct-upload session: %#v", got)
	}
	if got.StatusReason != modelsv1alpha1.ModelConditionReasonPending {
		t.Fatalf("unexpected status reason %q", got.StatusReason)
	}
	if got.RemoveFinalizer {
		t.Fatalf("did not expect finalizer removal while runtime cleanup is still needed: %#v", got)
	}
}

func TestFinalizeDeleteSnapshotsDirectUploadSessionBeforeRuntimeCleanup(t *testing.T) {
	t.Parallel()

	got := FinalizeDelete(FinalizeDeleteInput{
		HasFinalizer:           true,
		RuntimeResourcePresent: true,
		DirectUploadSession:    true,
	})

	if !got.EnsureGarbageCollectionRequest || !got.DeleteRuntimeResources || !got.Requeue {
		t.Fatalf("unexpected decision %#v", got)
	}
	if got.RemoveFinalizer {
		t.Fatalf("did not expect finalizer removal while runtime cleanup is still needed: %#v", got)
	}
}
