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

package oci

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type directUploadMemoryStore struct {
	state modelpackports.DirectUploadState
	found bool
}

func (s *directUploadMemoryStore) Load(context.Context) (modelpackports.DirectUploadState, bool, error) {
	return s.state, s.found, nil
}

func (s *directUploadMemoryStore) Save(_ context.Context, state modelpackports.DirectUploadState) error {
	s.state = state
	s.found = true
	return nil
}

func TestPushRawLayerDirectToBackingStorageResumesPersistedSession(t *testing.T) {
	t.Parallel()

	registry, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{partSizeBytes: 64})
	layerPayload := append([]byte("GGUF"), bytes.Repeat([]byte("x"), 252)...)

	layerPath := filepath.Join(t.TempDir(), "weights.gguf")
	if err := os.WriteFile(layerPath, layerPayload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	layer := modelpackports.PublishLayer{
		SourcePath:  layerPath,
		TargetPath:  "model/weights.gguf",
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatRaw,
		Compression: modelpackports.LayerCompressionNone,
	}
	plan, err := planPublishLayer(layer)
	if err != nil {
		t.Fatalf("planPublishLayer() error = %v", err)
	}

	firstChunk := layerPayload[:64]
	seedDirectUploadSession(t, directUpload, firstChunk)

	hasher := sha256.New()
	if _, err := hasher.Write(firstChunk); err != nil {
		t.Fatalf("hasher.Write() error = %v", err)
	}
	digestState, err := marshalDirectUploadDigestState(hasher)
	if err != nil {
		t.Fatalf("marshalDirectUploadDigestState() error = %v", err)
	}

	store := newRunningDirectUploadStore(plan, int64(len(layerPayload)), 64, digestState)

	descriptor, err := pushRawLayerDirectToBackingStorage(
		context.Background(),
		nil,
		withDirectUploadInput(modelpackports.PublishInput{
			ArtifactURI:       serverReference(registry.server, "published"),
			DirectUploadState: store,
		}, directUpload),
		auth,
		layer,
		plan,
		&directUploadCheckpoint{store: store, state: store.state},
		nil,
	)
	if err != nil {
		t.Fatalf("pushRawLayerDirectToBackingStorage() error = %v", err)
	}

	assertDirectUploadResumeResult(t, directUpload, store, 3)
	if got := descriptor.Size; got != int64(len(layerPayload)) {
		t.Fatalf("descriptor.Size = %d, want %d", got, len(layerPayload))
	}
}

func TestPushDescribedLayerDirectToBackingStorageResumesPersistedSession(t *testing.T) {
	t.Parallel()

	registry, directUpload, auth := newDirectPublishHarness(t, directUploadTestOptions{partSizeBytes: 64})

	modelDir := t.TempDir()
	writeTestModelFiles(t, modelDir, string(bytes.Repeat([]byte("a"), 128)), "GGUF"+string(bytes.Repeat([]byte("b"), 256)))

	layer := modelpackports.PublishLayer{
		SourcePath:  modelDir,
		TargetPath:  materializedLayerPath,
		Base:        modelpackports.LayerBaseModel,
		Format:      modelpackports.LayerFormatTar,
		Compression: modelpackports.LayerCompressionNone,
	}
	descriptor, err := describePublishLayer(context.Background(), layer)
	if err != nil {
		t.Fatalf("describePublishLayer() error = %v", err)
	}

	firstChunk, err := readPublishLayerChunk(context.Background(), layer, 0, 64)
	if err != nil {
		t.Fatalf("readPublishLayerChunk() error = %v", err)
	}
	seedDirectUploadSession(t, directUpload, firstChunk)

	store := newRunningDirectUploadStore(descriptor, descriptor.Size, int64(len(firstChunk)), nil)

	if err := pushDescribedLayerDirectToBackingStorage(
		context.Background(),
		nil,
		withDirectUploadInput(modelpackports.PublishInput{
			ArtifactURI:       serverReference(registry.server, "published"),
			DirectUploadState: store,
		}, directUpload),
		auth,
		layer,
		descriptor,
		&directUploadCheckpoint{store: store, state: store.state},
		nil,
	); err != nil {
		t.Fatalf("pushDescribedLayerDirectToBackingStorage() error = %v", err)
	}

	assertDirectUploadResumeResult(t, directUpload, store, int((descriptor.Size-int64(len(firstChunk))+63)/64))
}

func seedDirectUploadSession(t *testing.T, directUpload *directUploadTestServer, firstChunk []byte) {
	t.Helper()

	firstDigest := sha256.Sum256(firstChunk)
	directUpload.sessions["session-1"] = &directUploadSessionState{
		repository: "published",
		parts: map[int]uploadedDirectPart{
			1: {PartNumber: 1, ETag: hex.EncodeToString(firstDigest[:]), SizeBytes: int64(len(firstChunk))},
		},
		payloads: map[int][]byte{
			1: append([]byte(nil), firstChunk...),
		},
	}
	directUpload.nextSessionID = 1
}

func newRunningDirectUploadStore(
	descriptor publishLayerDescriptor,
	totalSizeBytes int64,
	uploadedSizeBytes int64,
	digestState []byte,
) *directUploadMemoryStore {
	return &directUploadMemoryStore{
		found: true,
		state: modelpackports.DirectUploadState{
			Phase: modelpackports.DirectUploadStatePhaseRunning,
			CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
				Key:               directUploadLayerKey(descriptor),
				SessionToken:      "session-1",
				PartSizeBytes:     64,
				TotalSizeBytes:    totalSizeBytes,
				UploadedSizeBytes: uploadedSizeBytes,
				DigestState:       digestState,
			},
		},
	}
}

func assertDirectUploadResumeResult(
	t *testing.T,
	directUpload *directUploadTestServer,
	store *directUploadMemoryStore,
	wantUploadCalls int,
) {
	t.Helper()

	if got, want := directUpload.listPartsCalls, 1; got != want {
		t.Fatalf("listPartsCalls = %d, want %d", got, want)
	}
	if got := directUpload.uploadCalls; got != wantUploadCalls {
		t.Fatalf("uploadCalls = %d, want %d after resume from first part", got, wantUploadCalls)
	}
	if got, want := directUpload.completeCalls, 1; got != want {
		t.Fatalf("completeCalls = %d, want %d", got, want)
	}
	if len(store.state.CompletedLayers) != 1 || store.state.CurrentLayer != nil {
		t.Fatalf("unexpected persisted state %#v", store.state)
	}
}

func TestDirectUploadCheckpointEnsureProgressPlanPersistsAndReusesPlan(t *testing.T) {
	t.Parallel()

	store := &directUploadMemoryStore{
		found: true,
		state: modelpackports.DirectUploadState{
			Phase: modelpackports.DirectUploadStatePhaseIdle,
			Stage: modelpackports.DirectUploadStateStageIdle,
		},
	}
	checkpoint := &directUploadCheckpoint{store: store, state: store.state}

	if err := checkpoint.ensureProgressPlan(context.Background(), 3, 1024); err != nil {
		t.Fatalf("ensureProgressPlan() first call error = %v", err)
	}
	if checkpoint.state.PlannedLayerCount != 3 || checkpoint.state.PlannedSizeBytes != 1024 {
		t.Fatalf("unexpected checkpoint state %#v", checkpoint.state)
	}
	if store.state.PlannedLayerCount != 3 || store.state.PlannedSizeBytes != 1024 {
		t.Fatalf("unexpected persisted state %#v", store.state)
	}

	if err := checkpoint.ensureProgressPlan(context.Background(), 3, 1024); err != nil {
		t.Fatalf("ensureProgressPlan() second call error = %v", err)
	}
	if err := checkpoint.ensureProgressPlan(context.Background(), 4, 1024); err == nil {
		t.Fatal("expected ensureProgressPlan() to reject mismatched layer count")
	}
}
