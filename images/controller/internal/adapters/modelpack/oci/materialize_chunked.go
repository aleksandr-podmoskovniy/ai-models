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

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func extractChunkedLayers(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	destination string,
	layerSet modelPackLayerSet,
) error {
	indexPayload, err := FetchBlob(ctx, client, reference, layerSet.ChunkIndex.Digest, auth)
	if err != nil {
		return fmt.Errorf("failed to fetch ModelPack chunk index: %w", err)
	}
	index, err := decodeChunkIndex(indexPayload, layerSet.ChunkPacks)
	if err != nil {
		return err
	}
	packCache := map[string][]byte{}
	for _, file := range index.Files {
		if err := extractChunkedFile(ctx, client, reference, auth, destination, file, index.Packs, packCache); err != nil {
			return err
		}
	}
	return nil
}

func extractChunkedFile(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	destination string,
	file chunkIndexFile,
	packs []chunkIndexPack,
	packCache map[string][]byte,
) error {
	target, err := materializeTargetPath(destination, file.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		_ = out.Close()
		if !success {
			_ = os.Remove(target)
		}
	}()
	if err := out.Truncate(file.SizeBytes); err != nil {
		return err
	}
	for _, chunk := range file.Chunks {
		payload, err := loadStoredChunk(ctx, client, reference, auth, chunk, packs, packCache)
		if err != nil {
			return err
		}
		decoded, err := decodeStoredChunk(payload, chunk.Compression)
		if err != nil {
			return err
		}
		if int64(len(decoded)) != chunk.UncompressedSizeBytes {
			return fmt.Errorf("chunk index file %q chunk %d decoded size %d does not match expected %d", file.Path, chunk.Index, len(decoded), chunk.UncompressedSizeBytes)
		}
		if _, err := out.WriteAt(decoded, chunk.Offset); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := verifyFileDigest(target, file.Digest); err != nil {
		return err
	}
	success = true
	return nil
}

func loadStoredChunk(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	chunk chunkIndexChunk,
	packs []chunkIndexPack,
	packCache map[string][]byte,
) ([]byte, error) {
	pack, found := findChunkPack(packs, chunk.Pack)
	if !found {
		return nil, fmt.Errorf("chunk %d references missing pack %q", chunk.Index, chunk.Pack)
	}
	packPayload, found := packCache[pack.ID]
	if !found {
		payload, err := FetchBlob(ctx, client, reference, pack.Digest, auth)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch ModelPack chunk pack %q: %w", pack.ID, err)
		}
		packPayload = payload
		packCache[pack.ID] = payload
	}
	end := chunk.PackOffset + chunk.PackLength
	if chunk.PackOffset < 0 || end > int64(len(packPayload)) {
		return nil, fmt.Errorf("chunk %d pack range is outside fetched pack %q", chunk.Index, pack.ID)
	}
	stored := packPayload[int(chunk.PackOffset):int(end)]
	if err := verifyChunkDigest(stored, chunk.StoredDigest); err != nil {
		return nil, err
	}
	return stored, nil
}

func findChunkPack(packs []chunkIndexPack, id string) (chunkIndexPack, bool) {
	for _, pack := range packs {
		if pack.ID == id {
			return pack, true
		}
	}
	return chunkIndexPack{}, false
}
