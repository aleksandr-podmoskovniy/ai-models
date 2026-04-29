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
	"encoding/json"
	"os"
	"path/filepath"
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

func jsonBytes(t *testing.T, value any) []byte {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}
