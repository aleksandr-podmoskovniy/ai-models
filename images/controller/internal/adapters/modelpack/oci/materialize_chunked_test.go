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
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestMaterializerMaterializesChunkedModelPack(t *testing.T) {
	t.Parallel()

	modelPayload := []byte("chunked immutable model payload")
	chunkA := modelPayload[:11]
	chunkB := modelPayload[11:]
	packPayload := append(append([]byte{}, chunkA...), chunkB...)
	packDigest := sha256DigestBytes(packPayload)
	fileDigest := sha256DigestBytes(modelPayload)

	index := chunkIndex{
		SchemaVersion:  chunkIndexSchemaVersion,
		CreatedBy:      "test",
		ChunkSizeBytes: 11,
		Files: []chunkIndexFile{{
			Path:      "model/model.gguf",
			SizeBytes: int64(len(modelPayload)),
			Digest:    fileDigest,
			Chunks: []chunkIndexChunk{
				{
					Index:                 0,
					Offset:                0,
					UncompressedSizeBytes: int64(len(chunkA)),
					StoredSizeBytes:       int64(len(chunkA)),
					StoredDigest:          sha256DigestBytes(chunkA),
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            0,
					PackLength:            int64(len(chunkA)),
				},
				{
					Index:                 1,
					Offset:                int64(len(chunkA)),
					UncompressedSizeBytes: int64(len(chunkB)),
					StoredSizeBytes:       int64(len(chunkB)),
					StoredDigest:          sha256DigestBytes(chunkB),
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            int64(len(chunkA)),
					PackLength:            int64(len(chunkB)),
				},
			},
		}},
		Packs: []chunkIndexPack{{
			ID:        "chunk-pack-000",
			Digest:    packDigest,
			SizeBytes: int64(len(packPayload)),
			MediaType: ModelPackChunkPackMediaType,
		}},
	}
	indexPayload := jsonBytes(t, index)
	indexDigest := sha256DigestBytes(indexPayload)

	manifestBody := jsonBytes(t, map[string]any{
		"schemaVersion": 2,
		"artifactType":  ModelPackArtifactType,
		"config": map[string]any{
			"mediaType": ModelPackConfigMediaType,
			"digest":    "sha256:config",
			"size":      10,
		},
		"layers": []map[string]any{
			{
				"mediaType": ModelPackChunkIndexMediaType,
				"digest":    indexDigest,
				"size":      len(indexPayload),
			},
			{
				"mediaType": ModelPackChunkPackMediaType,
				"digest":    packDigest,
				"size":      len(packPayload),
			},
		},
	})
	configBody := jsonBytes(t, map[string]any{
		"descriptor": map[string]any{"name": "model"},
		"modelfs": map[string]any{
			"type":    "chunked",
			"diffIds": []string{indexDigest, packDigest},
		},
		"config": map[string]any{},
	})

	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		manifestBody: manifestBody,
		configBody:   configBody,
		blobBodies: map[string][]byte{
			"sha256:config": configBody,
			indexDigest:     indexPayload,
			packDigest:      packPayload,
		},
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "current")
	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	if got, want := result.ModelPath, filepath.Join(destination, "model"); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	materialized, err := os.ReadFile(filepath.Join(result.ModelPath, "model.gguf"))
	if err != nil {
		t.Fatalf("ReadFile(materialized model) error = %v", err)
	}
	if string(materialized) != string(modelPayload) {
		t.Fatalf("materialized payload = %q, want %q", string(materialized), string(modelPayload))
	}
}

func TestMaterializerUsesRangeRequestsForChunkedPack(t *testing.T) {
	t.Parallel()

	fixture := newChunkedMaterializeFixture(t)
	var rangeCalls int64
	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		manifestBody: fixture.manifestBody,
		configBody:   fixture.configBody,
		blobBodies:   fixture.blobBodies(),
		blobHandlers: map[string]func(http.ResponseWriter, *http.Request, []byte) bool{
			fixture.packDigest: func(w http.ResponseWriter, r *http.Request, payload []byte) bool {
				atomic.AddInt64(&rangeCalls, 1)
				return serveTestBlobRange(w, r, payload)
			},
		},
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "current")
	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	assertChunkedPayload(t, result.ModelPath, fixture.modelPayload)
	if got := atomic.LoadInt64(&rangeCalls); got != 2 {
		t.Fatalf("range calls = %d, want 2", got)
	}
}

func TestMaterializerFailsOnUnsatisfiableChunkRange(t *testing.T) {
	t.Parallel()

	fixture := newChunkedMaterializeFixture(t)
	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		manifestBody: fixture.manifestBody,
		configBody:   fixture.configBody,
		blobBodies:   fixture.blobBodies(),
		blobHandlers: map[string]func(http.ResponseWriter, *http.Request, []byte) bool{
			fixture.packDigest: func(w http.ResponseWriter, _ *http.Request, _ []byte) bool {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				_, _ = w.Write([]byte("range gone"))
				return true
			},
		},
	})
	defer server.Close()

	_, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: filepath.Join(t.TempDir(), "current"),
	}, auth)
	if err == nil || !strings.Contains(err.Error(), "not satisfiable") {
		t.Fatalf("Materialize() error = %v, want not satisfiable", err)
	}
}

func TestMaterializerResumesCompletedChunk(t *testing.T) {
	t.Parallel()

	fixture := newChunkedMaterializeFixture(t)
	destination := filepath.Join(t.TempDir(), "current")
	workRoot := chunkMaterializeWorkRoot(destination)
	target := filepath.Join(workRoot, "model", "model.gguf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, fixture.chunkA, 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(target, os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(int64(len(fixture.modelPayload))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := prepareChunkMaterializeWorkRoot(workRoot, "sha256:deadbeef"); err != nil {
		t.Fatal(err)
	}
	state := newChunkMaterializeState(nil, "", modelpackports.RegistryAuth{}, workRoot, fixture.index.Packs)
	if err := state.markChunkComplete(fixture.index.Files[0].Path, fixture.index.Files[0].Chunks[0]); err != nil {
		t.Fatal(err)
	}

	var requestedRanges []string
	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		manifestBody: fixture.manifestBody,
		configBody:   fixture.configBody,
		blobBodies:   fixture.blobBodies(),
		blobHandlers: map[string]func(http.ResponseWriter, *http.Request, []byte) bool{
			fixture.packDigest: func(w http.ResponseWriter, r *http.Request, payload []byte) bool {
				requestedRanges = append(requestedRanges, r.Header.Get("Range"))
				return serveTestBlobRange(w, r, payload)
			},
		},
	})
	defer server.Close()

	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	assertChunkedPayload(t, result.ModelPath, fixture.modelPayload)
	if got, want := strings.Join(requestedRanges, ","), fmt.Sprintf("bytes=%d-%d", len(fixture.chunkA), len(fixture.modelPayload)-1); got != want {
		t.Fatalf("requested ranges = %q, want %q", got, want)
	}
}

func TestMaterializerResumesVerifiedFilesWhenLaterFileFails(t *testing.T) {
	t.Parallel()

	fixture := newTwoFileChunkedMaterializeFixture(t)
	destination := filepath.Join(t.TempDir(), "current")
	var firstFileRangeCalls int64
	var secondFileRangeCalls int64
	var failSecondFileRange int64 = 1
	secondFileRangePrefix := fmt.Sprintf("bytes=%d-", len(fixture.modelPayload))
	server, auth, _ := newModelPackTestServer(t, modelPackServerOptions{
		manifestBody: fixture.manifestBody,
		configBody:   fixture.configBody,
		blobBodies:   fixture.blobBodies(),
		blobHandlers: map[string]func(http.ResponseWriter, *http.Request, []byte) bool{
			fixture.packDigest: func(w http.ResponseWriter, r *http.Request, payload []byte) bool {
				rangeHeader := r.Header.Get("Range")
				if strings.HasPrefix(rangeHeader, "bytes=0-") {
					atomic.AddInt64(&firstFileRangeCalls, 1)
				}
				if strings.HasPrefix(rangeHeader, secondFileRangePrefix) {
					atomic.AddInt64(&secondFileRangeCalls, 1)
					if atomic.CompareAndSwapInt64(&failSecondFileRange, 1, 0) {
						w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
						_, _ = w.Write([]byte("second range temporarily unavailable"))
						return true
					}
				}
				return serveTestBlobRange(w, r, payload)
			},
		},
	})
	defer server.Close()

	_, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err == nil || !strings.Contains(err.Error(), "not satisfiable") {
		t.Fatalf("first Materialize() error = %v, want temporary range failure", err)
	}

	result, err := NewMaterializer().Materialize(context.Background(), modelpackports.MaterializeInput{
		ArtifactURI:    serverReference(server, "@sha256:deadbeef"),
		DestinationDir: destination,
		ArtifactFamily: "gguf-v1",
	}, auth)
	if err != nil {
		t.Fatalf("second Materialize() error = %v", err)
	}
	assertMaterializedFile(t, filepath.Join(result.ModelPath, "first.gguf"), fixture.modelPayload)
	assertMaterializedFile(t, filepath.Join(result.ModelPath, "second.gguf"), fixture.secondPayload)
	if got, want := atomic.LoadInt64(&firstFileRangeCalls), int64(1); got != want {
		t.Fatalf("first file range calls = %d, want %d after retry", got, want)
	}
	if got, want := atomic.LoadInt64(&secondFileRangeCalls), int64(2); got != want {
		t.Fatalf("second file range calls = %d, want %d", got, want)
	}
}
