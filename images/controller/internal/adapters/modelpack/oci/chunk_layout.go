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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/deckhouse/ai-models/controller/internal/support/archiveio"
)

const (
	ModelPackChunkIndexMediaType    = "application/vnd.deckhouse.ai-models.modelpack.chunk-index.v1+json"
	ModelPackChunkPackMediaType     = "application/vnd.deckhouse.ai-models.modelpack.chunk-pack.v1"
	ModelPackChunkPackZstdMediaType = "application/vnd.deckhouse.ai-models.modelpack.chunk-pack.zstd.v1"

	chunkIndexSchemaVersion = "modelpack.chunked.v1"
	chunkCompressionNone    = "none"
	chunkCompressionZstd    = "zstd"
)

type modelPackLayerSet struct {
	Legacy     []publishLayerDescriptor
	ChunkIndex *chunkLayerDescriptor
	ChunkPacks map[string]chunkLayerDescriptor
}

func (s modelPackLayerSet) Chunked() bool {
	return s.ChunkIndex != nil
}

type chunkLayerDescriptor struct {
	Digest    string
	Size      int64
	MediaType string
}

type chunkIndex struct {
	SchemaVersion  string           `json:"schemaVersion"`
	CreatedBy      string           `json:"createdBy,omitempty"`
	ChunkSizeBytes int64            `json:"chunkSizeBytes"`
	Files          []chunkIndexFile `json:"files"`
	Packs          []chunkIndexPack `json:"packs"`
}

type chunkIndexFile struct {
	Path      string            `json:"path"`
	SizeBytes int64             `json:"sizeBytes"`
	Digest    string            `json:"digest"`
	Source    *chunkIndexSource `json:"source,omitempty"`
	Chunks    []chunkIndexChunk `json:"chunks"`
}

type chunkIndexSource struct {
	Kind       string `json:"kind,omitempty"`
	URI        string `json:"uri,omitempty"`
	ETag       string `json:"etag,omitempty"`
	Generation string `json:"generation,omitempty"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
}

type chunkIndexChunk struct {
	Index                 int    `json:"index"`
	Offset                int64  `json:"offset"`
	UncompressedSizeBytes int64  `json:"uncompressedSizeBytes"`
	StoredSizeBytes       int64  `json:"storedSizeBytes"`
	StoredDigest          string `json:"storedDigest"`
	Compression           string `json:"compression"`
	Pack                  string `json:"pack"`
	PackOffset            int64  `json:"packOffset"`
	PackLength            int64  `json:"packLength"`
}

type chunkIndexPack struct {
	ID        string `json:"id"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes"`
	MediaType string `json:"mediaType"`
}

func classifyManifestLayers(layers []any) (modelPackLayerSet, error) {
	result := modelPackLayerSet{ChunkPacks: map[string]chunkLayerDescriptor{}}
	for index, layer := range layers {
		layerMap, _ := layer.(map[string]any)
		if layerMap == nil {
			return modelPackLayerSet{}, fmt.Errorf("registry manifest layer %d is invalid", index)
		}
		mediaType := strings.TrimSpace(stringValue(layerMap["mediaType"]))
		if isChunkIndexMediaType(mediaType) {
			if result.ChunkIndex != nil {
				return modelPackLayerSet{}, errors.New("registry manifest must contain at most one chunk index layer")
			}
			descriptor, err := decodeChunkLayerDescriptor(index, layerMap)
			if err != nil {
				return modelPackLayerSet{}, err
			}
			result.ChunkIndex = &descriptor
			continue
		}
		if isChunkPackMediaType(mediaType) {
			descriptor, err := decodeChunkLayerDescriptor(index, layerMap)
			if err != nil {
				return modelPackLayerSet{}, err
			}
			if _, found := result.ChunkPacks[descriptor.Digest]; found {
				return modelPackLayerSet{}, fmt.Errorf("registry manifest contains duplicate chunk pack digest %q", descriptor.Digest)
			}
			result.ChunkPacks[descriptor.Digest] = descriptor
			continue
		}
		descriptor, err := decodeLegacyManifestLayer(index, layerMap)
		if err != nil {
			return modelPackLayerSet{}, err
		}
		result.Legacy = append(result.Legacy, descriptor)
	}
	if result.ChunkIndex == nil && len(result.ChunkPacks) > 0 {
		return modelPackLayerSet{}, errors.New("registry manifest chunk pack layers require a chunk index layer")
	}
	if result.ChunkIndex != nil && len(result.ChunkPacks) == 0 {
		return modelPackLayerSet{}, errors.New("registry manifest chunk index requires at least one chunk pack layer")
	}
	return result, nil
}

func decodeChunkLayerDescriptor(index int, layerMap map[string]any) (chunkLayerDescriptor, error) {
	digest := strings.TrimSpace(stringValue(layerMap["digest"]))
	if digest == "" {
		return chunkLayerDescriptor{}, fmt.Errorf("registry manifest layer %d is missing digest", index)
	}
	size := parseSize(layerMap["size"])
	if size <= 0 {
		return chunkLayerDescriptor{}, fmt.Errorf("registry manifest layer %d size must be positive", index)
	}
	return chunkLayerDescriptor{
		Digest:    digest,
		Size:      size,
		MediaType: strings.TrimSpace(stringValue(layerMap["mediaType"])),
	}, nil
}

func isChunkIndexMediaType(value string) bool {
	return strings.TrimSpace(value) == ModelPackChunkIndexMediaType
}

func isChunkPackMediaType(value string) bool {
	switch strings.TrimSpace(value) {
	case ModelPackChunkPackMediaType, ModelPackChunkPackZstdMediaType:
		return true
	default:
		return false
	}
}

func decodeChunkIndex(payload []byte, manifestPacks map[string]chunkLayerDescriptor) (chunkIndex, error) {
	var index chunkIndex
	if err := json.Unmarshal(payload, &index); err != nil {
		return chunkIndex{}, fmt.Errorf("failed to decode ModelPack chunk index: %w", err)
	}
	if err := validateChunkIndex(index, manifestPacks); err != nil {
		return chunkIndex{}, err
	}
	return index, nil
}

func validateChunkIndex(index chunkIndex, manifestPacks map[string]chunkLayerDescriptor) error {
	if strings.TrimSpace(index.SchemaVersion) != chunkIndexSchemaVersion {
		return fmt.Errorf("chunk index schemaVersion must be %q", chunkIndexSchemaVersion)
	}
	if index.ChunkSizeBytes <= 0 {
		return errors.New("chunk index chunkSizeBytes must be positive")
	}
	packsByID, err := validateChunkIndexPacks(index.Packs, manifestPacks)
	if err != nil {
		return err
	}
	if len(index.Files) == 0 {
		return errors.New("chunk index must contain at least one file")
	}
	seenPaths := map[string]struct{}{}
	for _, file := range index.Files {
		relativePath, err := archiveio.RelativePath(file.Path)
		if err != nil {
			return fmt.Errorf("chunk index file path %q is invalid: %w", file.Path, err)
		}
		if relativePath == "." {
			return errors.New("chunk index file path must not be root")
		}
		if _, found := seenPaths[filepath.ToSlash(relativePath)]; found {
			return fmt.Errorf("chunk index duplicate file path %q", file.Path)
		}
		seenPaths[filepath.ToSlash(relativePath)] = struct{}{}
		if err := validateChunkIndexFile(file, packsByID); err != nil {
			return err
		}
	}
	return nil
}

func validateChunkIndexPacks(packs []chunkIndexPack, manifestPacks map[string]chunkLayerDescriptor) (map[string]chunkIndexPack, error) {
	if len(packs) == 0 {
		return nil, errors.New("chunk index must contain at least one pack")
	}
	result := map[string]chunkIndexPack{}
	seenDigests := map[string]struct{}{}
	for _, pack := range packs {
		pack.ID = strings.TrimSpace(pack.ID)
		pack.Digest = strings.TrimSpace(pack.Digest)
		pack.MediaType = strings.TrimSpace(pack.MediaType)
		if pack.ID == "" {
			return nil, errors.New("chunk index pack id must not be empty")
		}
		if _, found := result[pack.ID]; found {
			return nil, fmt.Errorf("chunk index duplicate pack id %q", pack.ID)
		}
		if pack.Digest == "" {
			return nil, fmt.Errorf("chunk index pack %q digest must not be empty", pack.ID)
		}
		if pack.SizeBytes <= 0 {
			return nil, fmt.Errorf("chunk index pack %q sizeBytes must be positive", pack.ID)
		}
		if !isChunkPackMediaType(pack.MediaType) {
			return nil, fmt.Errorf("chunk index pack %q has unsupported mediaType %q", pack.ID, pack.MediaType)
		}
		manifestPack, found := manifestPacks[pack.Digest]
		if !found {
			return nil, fmt.Errorf("chunk index pack %q is not present in manifest layers", pack.ID)
		}
		if manifestPack.Size != pack.SizeBytes {
			return nil, fmt.Errorf("chunk index pack %q sizeBytes %d does not match manifest size %d", pack.ID, pack.SizeBytes, manifestPack.Size)
		}
		if manifestPack.MediaType != pack.MediaType {
			return nil, fmt.Errorf("chunk index pack %q mediaType %q does not match manifest mediaType %q", pack.ID, pack.MediaType, manifestPack.MediaType)
		}
		if _, found := seenDigests[pack.Digest]; found {
			return nil, fmt.Errorf("chunk index duplicate pack digest %q", pack.Digest)
		}
		seenDigests[pack.Digest] = struct{}{}
		result[pack.ID] = pack
	}
	if len(result) != len(manifestPacks) {
		return nil, errors.New("chunk index packs must match manifest chunk pack layers")
	}
	return result, nil
}

func validateChunkIndexFile(file chunkIndexFile, packsByID map[string]chunkIndexPack) error {
	if file.SizeBytes <= 0 {
		return fmt.Errorf("chunk index file %q sizeBytes must be positive", file.Path)
	}
	if strings.TrimSpace(file.Digest) == "" {
		return fmt.Errorf("chunk index file %q digest must not be empty", file.Path)
	}
	if len(file.Chunks) == 0 {
		return fmt.Errorf("chunk index file %q must contain at least one chunk", file.Path)
	}
	chunks := append([]chunkIndexChunk(nil), file.Chunks...)
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Offset < chunks[j].Offset
	})
	var expectedOffset int64
	for position, chunk := range chunks {
		if chunk.Index != position {
			return fmt.Errorf("chunk index file %q chunk index %d must be %d", file.Path, chunk.Index, position)
		}
		if chunk.Offset != expectedOffset {
			return fmt.Errorf("chunk index file %q has gap or overlap at offset %d", file.Path, chunk.Offset)
		}
		if err := validateChunkIndexChunk(file.Path, chunk, packsByID); err != nil {
			return err
		}
		expectedOffset += chunk.UncompressedSizeBytes
	}
	if expectedOffset != file.SizeBytes {
		return fmt.Errorf("chunk index file %q chunks cover %d bytes, want %d", file.Path, expectedOffset, file.SizeBytes)
	}
	return nil
}

func validateChunkIndexChunk(filePath string, chunk chunkIndexChunk, packsByID map[string]chunkIndexPack) error {
	if chunk.UncompressedSizeBytes <= 0 {
		return fmt.Errorf("chunk index file %q chunk %d uncompressedSizeBytes must be positive", filePath, chunk.Index)
	}
	if chunk.StoredSizeBytes <= 0 {
		return fmt.Errorf("chunk index file %q chunk %d storedSizeBytes must be positive", filePath, chunk.Index)
	}
	if strings.TrimSpace(chunk.StoredDigest) == "" {
		return fmt.Errorf("chunk index file %q chunk %d storedDigest must not be empty", filePath, chunk.Index)
	}
	switch strings.TrimSpace(chunk.Compression) {
	case "", chunkCompressionNone, chunkCompressionZstd:
	default:
		return fmt.Errorf("chunk index file %q chunk %d has unsupported compression %q", filePath, chunk.Index, chunk.Compression)
	}
	pack, found := packsByID[strings.TrimSpace(chunk.Pack)]
	if !found {
		return fmt.Errorf("chunk index file %q chunk %d references missing pack %q", filePath, chunk.Index, chunk.Pack)
	}
	if chunk.PackOffset < 0 || chunk.PackLength <= 0 {
		return fmt.Errorf("chunk index file %q chunk %d pack range must be positive", filePath, chunk.Index)
	}
	if chunk.PackOffset+chunk.PackLength > pack.SizeBytes {
		return fmt.Errorf("chunk index file %q chunk %d pack range exceeds pack size", filePath, chunk.Index)
	}
	if chunk.StoredSizeBytes != chunk.PackLength {
		return fmt.Errorf("chunk index file %q chunk %d storedSizeBytes must match packLength", filePath, chunk.Index)
	}
	return nil
}

func sha256DigestBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func verifyChunkDigest(payload []byte, digest string) error {
	if got := sha256DigestBytes(payload); got != strings.TrimSpace(digest) {
		return fmt.Errorf("chunk digest mismatch: expected %q, got %q", strings.TrimSpace(digest), got)
	}
	return nil
}

func verifyFileDigest(path, digest string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	got := "sha256:" + hex.EncodeToString(hasher.Sum(nil))
	if got != strings.TrimSpace(digest) {
		return fmt.Errorf("materialized file digest mismatch for %q: expected %q, got %q", path, strings.TrimSpace(digest), got)
	}
	return nil
}

func decodeStoredChunk(payload []byte, compression string) ([]byte, error) {
	switch strings.TrimSpace(compression) {
	case "", chunkCompressionNone:
		return payload, nil
	case chunkCompressionZstd:
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer decoder.Close()
		return decoder.DecodeAll(payload, nil)
	default:
		return nil, fmt.Errorf("unsupported chunk compression %q", compression)
	}
}
