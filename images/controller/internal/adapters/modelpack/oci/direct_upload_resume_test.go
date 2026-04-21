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
	firstDigest := sha256.Sum256(firstChunk)
	directUpload.sessions["session-1"] = &directUploadSessionState{
		repository: "published",
		parts: map[int]uploadedDirectPart{
			1: {
				PartNumber: 1,
				ETag:       hex.EncodeToString(firstDigest[:]),
				SizeBytes:  int64(len(firstChunk)),
			},
		},
		payloads: map[int][]byte{
			1: append([]byte(nil), firstChunk...),
		},
	}
	directUpload.nextSessionID = 1

	hasher := sha256.New()
	if _, err := hasher.Write(firstChunk); err != nil {
		t.Fatalf("hasher.Write() error = %v", err)
	}
	digestState, err := marshalDirectUploadDigestState(hasher)
	if err != nil {
		t.Fatalf("marshalDirectUploadDigestState() error = %v", err)
	}

	store := &directUploadMemoryStore{
		found: true,
		state: modelpackports.DirectUploadState{
			Phase: modelpackports.DirectUploadStatePhaseRunning,
			CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
				Key:               directUploadLayerKey(plan),
				SessionToken:      "session-1",
				PartSizeBytes:     64,
				TotalSizeBytes:    int64(len(layerPayload)),
				UploadedSizeBytes: 64,
				DigestState:       digestState,
			},
		},
	}

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
	)
	if err != nil {
		t.Fatalf("pushRawLayerDirectToBackingStorage() error = %v", err)
	}

	if got, want := directUpload.listPartsCalls, 1; got != want {
		t.Fatalf("listPartsCalls = %d, want %d", got, want)
	}
	if got, want := directUpload.uploadCalls, 3; got != want {
		t.Fatalf("uploadCalls = %d, want %d after resume from first part", got, want)
	}
	if got, want := directUpload.completeCalls, 1; got != want {
		t.Fatalf("completeCalls = %d, want %d", got, want)
	}
	if got := descriptor.Size; got != int64(len(layerPayload)) {
		t.Fatalf("descriptor.Size = %d, want %d", got, len(layerPayload))
	}
	if len(store.state.CompletedLayers) != 1 || store.state.CurrentLayer != nil {
		t.Fatalf("unexpected persisted state %#v", store.state)
	}
}
