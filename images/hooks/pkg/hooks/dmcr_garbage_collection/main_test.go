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

package dmcr_garbage_collection

import (
	"context"
	"testing"

	"github.com/deckhouse/module-sdk/pkg"
	patchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"
	sdktest "github.com/deckhouse/module-sdk/testing/mock"
)

func TestHandleDMCRGarbageCollectionSetsDMCRObject(t *testing.T) {
	t.Parallel()

	values, err := patchablevalues.NewPatchableValues(map[string]any{
		"aiModels": map[string]any{
			"internal": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Expect(secretSnapshotName).Return(nil)

	input := &pkg.HookInput{
		Snapshots: snapshots,
		Values:    values,
	}

	if err := handleDMCRGarbageCollection(context.Background(), input); err != nil {
		t.Fatalf("handleDMCRGarbageCollection: %v", err)
	}

	patches := values.GetPatches()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	if patches[0].Path != "/aiModels/internal/dmcr" {
		t.Fatalf("unexpected patch path: %s", patches[0].Path)
	}

	if got := string(patches[0].Value); got != `{"garbageCollectionModeEnabled":false}` {
		t.Fatalf("unexpected patch value: %s", got)
	}
}

func TestHandleDMCRGarbageCollectionEnablesModeWhenSwitchSecretExists(t *testing.T) {
	t.Parallel()

	values, err := patchablevalues.NewPatchableValues(map[string]any{
		"aiModels": map[string]any{
			"internal": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		secret, ok := v.(*partialSecret)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}

		*secret = partialSecret{
			Metadata: partialSecretMetadata{
				Labels: map[string]string{
					requestLabelKey: requestLabelValue,
				},
				Annotations: map[string]string{
					switchAnnotationKey: "enabled",
				},
			},
		}
		return nil
	})
	snapshot.StringMock.Optional().Return("")

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Expect(secretSnapshotName).Return([]pkg.Snapshot{snapshot})

	input := &pkg.HookInput{
		Snapshots: snapshots,
		Values:    values,
	}

	if err := handleDMCRGarbageCollection(context.Background(), input); err != nil {
		t.Fatalf("handleDMCRGarbageCollection: %v", err)
	}

	patches := values.GetPatches()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	if got := string(patches[0].Value); got != `{"garbageCollectionModeEnabled":true}` {
		t.Fatalf("unexpected patch value: %s", got)
	}
}

func TestHandleDMCRGarbageCollectionKeepsModeDisabledForQueuedRequest(t *testing.T) {
	t.Parallel()

	values, err := patchablevalues.NewPatchableValues(map[string]any{
		"aiModels": map[string]any{
			"internal": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("new patchable values: %v", err)
	}

	snapshot := sdktest.NewSnapshotMock(t)
	snapshot.UnmarshalToMock.Set(func(v any) error {
		secret, ok := v.(*partialSecret)
		if !ok {
			t.Fatalf("unexpected snapshot target type %T", v)
		}

		*secret = partialSecret{
			Metadata: partialSecretMetadata{
				Labels: map[string]string{
					requestLabelKey: requestLabelValue,
				},
				Annotations: map[string]string{
					"ai.deckhouse.io/dmcr-gc-requested-at": "2026-04-16T12:00:00Z",
				},
			},
		}
		return nil
	})
	snapshot.StringMock.Optional().Return("")

	snapshots := sdktest.NewSnapshotsMock(t)
	snapshots.GetMock.Expect(secretSnapshotName).Return([]pkg.Snapshot{snapshot})

	input := &pkg.HookInput{
		Snapshots: snapshots,
		Values:    values,
	}

	if err := handleDMCRGarbageCollection(context.Background(), input); err != nil {
		t.Fatalf("handleDMCRGarbageCollection: %v", err)
	}

	patches := values.GetPatches()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	if got := string(patches[0].Value); got != `{"garbageCollectionModeEnabled":false}` {
		t.Fatalf("unexpected patch value: %s", got)
	}
}
