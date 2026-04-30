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
	"strings"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestValidatePayloadAcceptsChunkedLayout(t *testing.T) {
	t.Parallel()

	err := ValidatePayload(InspectPayload{
		"digest": "sha256:deadbeef",
		"manifest": map[string]any{
			"schemaVersion": 2,
			"artifactType":  ModelPackArtifactType,
			"config": map[string]any{
				"mediaType": ModelPackConfigMediaType,
				"digest":    "sha256:config",
				"size":      10,
			},
			"layers": []any{
				map[string]any{
					"mediaType": ModelPackChunkIndexMediaType,
					"digest":    "sha256:index",
					"size":      11,
				},
				map[string]any{
					"mediaType": ModelPackChunkPackMediaType,
					"digest":    "sha256:pack",
					"size":      12,
				},
			},
		},
		"configBlob": map[string]any{
			"descriptor": map[string]any{"name": "model"},
			"modelfs": map[string]any{
				"type":    "chunked",
				"diffIds": []any{"sha256:index", "sha256:pack"},
			},
			"config": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("ValidatePayload() error = %v", err)
	}
}

func TestValidateChunkIndexRejectsOverlappingChunks(t *testing.T) {
	t.Parallel()

	index := validTestChunkIndex()
	index.Files[0].Chunks[1].Offset = 1
	err := validateChunkIndex(index, testManifestPacks(index))
	if err == nil || !strings.Contains(err.Error(), "gap or overlap") {
		t.Fatalf("expected overlap error, got %v", err)
	}
}

func TestValidateChunkIndexRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	index := validTestChunkIndex()
	index.Files[0].Path = "../model.gguf"
	err := validateChunkIndex(index, testManifestPacks(index))
	if err == nil || !strings.Contains(err.Error(), "outside of destination") {
		t.Fatalf("expected path traversal error, got %v", err)
	}
}

func TestValidateChunkIndexRejectsOversizedChunkPayload(t *testing.T) {
	t.Parallel()

	index := validTestChunkIndex()
	index.ChunkSizeBytes = maxChunkPayloadBytes + 1
	err := validateChunkIndex(index, testManifestPacks(index))
	if err == nil || !strings.Contains(err.Error(), "chunkSizeBytes must not exceed") {
		t.Fatalf("expected chunk size bound error, got %v", err)
	}
}

func TestValidateChunkIndexRejectsOverflowingPackRange(t *testing.T) {
	t.Parallel()

	index := validTestChunkIndex()
	index.Packs[0].SizeBytes = maxInt64
	index.Files[0].Chunks[0].PackOffset = maxInt64
	index.Files[0].Chunks[0].PackLength = 2
	index.Files[0].Chunks[0].StoredSizeBytes = 2
	err := validateChunkIndex(index, testManifestPacks(index))
	if err == nil || !strings.Contains(err.Error(), "pack range overflows int64") {
		t.Fatalf("expected overflow error, got %v", err)
	}
}

func TestFetchBlobRangeRejectsOverflowingRange(t *testing.T) {
	t.Parallel()

	_, _, err := FetchBlobRange(context.Background(), nil, "registry.example/repo@sha256:deadbeef", "sha256:pack", modelpackports.RegistryAuth{}, maxInt64, 2)
	if err == nil || !strings.Contains(err.Error(), "overflows int64") {
		t.Fatalf("expected range overflow error, got %v", err)
	}
}

func validTestChunkIndex() chunkIndex {
	return chunkIndex{
		SchemaVersion:  chunkIndexSchemaVersion,
		ChunkSizeBytes: 2,
		Files: []chunkIndexFile{{
			Path:      "model/model.gguf",
			SizeBytes: 4,
			Digest:    "sha256:file",
			Chunks: []chunkIndexChunk{
				{
					Index:                 0,
					Offset:                0,
					UncompressedSizeBytes: 2,
					StoredSizeBytes:       2,
					StoredDigest:          "sha256:a",
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            0,
					PackLength:            2,
				},
				{
					Index:                 1,
					Offset:                2,
					UncompressedSizeBytes: 2,
					StoredSizeBytes:       2,
					StoredDigest:          "sha256:b",
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            2,
					PackLength:            2,
				},
			},
		}},
		Packs: []chunkIndexPack{{
			ID:        "chunk-pack-000",
			Digest:    "sha256:pack",
			SizeBytes: 4,
			MediaType: ModelPackChunkPackMediaType,
		}},
	}
}

func testManifestPacks(index chunkIndex) map[string]chunkLayerDescriptor {
	result := map[string]chunkLayerDescriptor{}
	for _, pack := range index.Packs {
		result[pack.Digest] = chunkLayerDescriptor{
			Digest:    pack.Digest,
			Size:      pack.SizeBytes,
			MediaType: pack.MediaType,
		}
	}
	return result
}
