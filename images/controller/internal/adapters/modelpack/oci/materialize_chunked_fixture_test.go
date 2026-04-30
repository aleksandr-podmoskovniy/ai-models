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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type chunkedMaterializeFixture struct {
	modelPayload  []byte
	secondPayload []byte
	chunkA        []byte
	chunkB        []byte
	packPayload   []byte
	packDigest    string
	indexDigest   string
	index         chunkIndex
	indexPayload  []byte
	manifestBody  []byte
	configBody    []byte
}

func newChunkedMaterializeFixture(t *testing.T) chunkedMaterializeFixture {
	t.Helper()

	modelPayload := []byte("chunked immutable model payload")
	chunkA := modelPayload[:11]
	chunkB := modelPayload[11:]
	packPayload := append(append([]byte{}, chunkA...), chunkB...)
	packDigest := sha256DigestBytes(packPayload)
	index := chunkIndex{
		SchemaVersion:  chunkIndexSchemaVersion,
		CreatedBy:      "test",
		ChunkSizeBytes: 11,
		Files: []chunkIndexFile{{
			Path:      "model/model.gguf",
			SizeBytes: int64(len(modelPayload)),
			Digest:    sha256DigestBytes(modelPayload),
			Chunks: []chunkIndexChunk{
				testChunk(0, 0, chunkA, "chunk-pack-000"),
				testChunk(1, int64(len(chunkA)), chunkB, "chunk-pack-000"),
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
	fixture := chunkedFixtureFromIndex(t, modelPayload, nil, packPayload, packDigest, index, indexPayload)
	fixture.chunkA = chunkA
	fixture.chunkB = chunkB
	return fixture
}

func newTwoFileChunkedMaterializeFixture(t *testing.T) chunkedMaterializeFixture {
	t.Helper()

	firstPayload := []byte("first immutable model payload")
	secondPayload := []byte("second immutable model payload")
	packPayload := append(append([]byte{}, firstPayload...), secondPayload...)
	packDigest := sha256DigestBytes(packPayload)
	index := chunkIndex{
		SchemaVersion:  chunkIndexSchemaVersion,
		CreatedBy:      "test",
		ChunkSizeBytes: int64(len(firstPayload)),
		Files: []chunkIndexFile{
			{
				Path:      "model/first.gguf",
				SizeBytes: int64(len(firstPayload)),
				Digest:    sha256DigestBytes(firstPayload),
				Chunks: []chunkIndexChunk{{
					Index:                 0,
					Offset:                0,
					UncompressedSizeBytes: int64(len(firstPayload)),
					StoredSizeBytes:       int64(len(firstPayload)),
					StoredDigest:          sha256DigestBytes(firstPayload),
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            0,
					PackLength:            int64(len(firstPayload)),
				}},
			},
			{
				Path:      "model/second.gguf",
				SizeBytes: int64(len(secondPayload)),
				Digest:    sha256DigestBytes(secondPayload),
				Chunks: []chunkIndexChunk{{
					Index:                 0,
					Offset:                0,
					UncompressedSizeBytes: int64(len(secondPayload)),
					StoredSizeBytes:       int64(len(secondPayload)),
					StoredDigest:          sha256DigestBytes(secondPayload),
					Compression:           chunkCompressionNone,
					Pack:                  "chunk-pack-000",
					PackOffset:            int64(len(firstPayload)),
					PackLength:            int64(len(secondPayload)),
				}},
			},
		},
		Packs: []chunkIndexPack{{
			ID:        "chunk-pack-000",
			Digest:    packDigest,
			SizeBytes: int64(len(packPayload)),
			MediaType: ModelPackChunkPackMediaType,
		}},
	}
	indexPayload := jsonBytes(t, index)
	return chunkedFixtureFromIndex(t, firstPayload, secondPayload, packPayload, packDigest, index, indexPayload)
}

func testChunk(index int, offset int64, payload []byte, pack string) chunkIndexChunk {
	return chunkIndexChunk{
		Index:                 index,
		Offset:                offset,
		UncompressedSizeBytes: int64(len(payload)),
		StoredSizeBytes:       int64(len(payload)),
		StoredDigest:          sha256DigestBytes(payload),
		Compression:           chunkCompressionNone,
		Pack:                  pack,
		PackOffset:            offset,
		PackLength:            int64(len(payload)),
	}
}

func chunkedFixtureFromIndex(
	t *testing.T,
	modelPayload []byte,
	secondPayload []byte,
	packPayload []byte,
	packDigest string,
	index chunkIndex,
	indexPayload []byte,
) chunkedMaterializeFixture {
	t.Helper()

	indexDigest := sha256DigestBytes(indexPayload)
	return chunkedMaterializeFixture{
		modelPayload:  modelPayload,
		secondPayload: secondPayload,
		packPayload:   packPayload,
		packDigest:    packDigest,
		indexDigest:   indexDigest,
		index:         index,
		indexPayload:  indexPayload,
		manifestBody: jsonBytes(t, map[string]any{
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
		}),
		configBody: jsonBytes(t, map[string]any{
			"descriptor": map[string]any{"name": "model"},
			"modelfs": map[string]any{
				"type":    "chunked",
				"diffIds": []string{indexDigest, packDigest},
			},
			"config": map[string]any{},
		}),
	}
}

func (f chunkedMaterializeFixture) blobBodies() map[string][]byte {
	return map[string][]byte{
		"sha256:config": f.configBody,
		f.indexDigest:   f.indexPayload,
		f.packDigest:    f.packPayload,
	}
}

func serveTestBlobRange(w http.ResponseWriter, r *http.Request, payload []byte) bool {
	rangeHeader := strings.TrimSpace(r.Header.Get("Range"))
	if rangeHeader == "" {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("missing range"))
		return true
	}
	cleanRange := strings.TrimPrefix(rangeHeader, "bytes=")
	startRaw, endRaw, found := strings.Cut(cleanRange, "-")
	if !found {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	start, err := strconv.ParseInt(startRaw, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return true
	}
	end, err := strconv.ParseInt(endRaw, 10, 64)
	if err != nil || start < 0 || end < start || end >= int64(len(payload)) {
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return true
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(payload[start : end+1])
	return true
}

func assertChunkedPayload(t *testing.T, modelPath string, want []byte) {
	t.Helper()

	assertMaterializedFile(t, filepath.Join(modelPath, "model.gguf"), want)
}

func assertMaterializedFile(t *testing.T, path string, want []byte) {
	t.Helper()

	materialized, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(materialized model) error = %v", err)
	}
	if string(materialized) != string(want) {
		t.Fatalf("materialized payload = %q, want %q", string(materialized), string(want))
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
