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
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"golang.org/x/sync/errgroup"
)

const chunkedMaterializeWorkers = 4

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
	state := newChunkMaterializeState(client, reference, auth, destination, index.Packs)
	for _, file := range index.Files {
		if err := extractChunkedFile(ctx, destination, file, state); err != nil {
			return err
		}
	}
	return nil
}

func extractChunkedFile(
	ctx context.Context,
	destination string,
	file chunkIndexFile,
	state *chunkMaterializeState,
) error {
	target, err := materializeTargetPath(destination, file.Path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = out.Close()
		}
	}()
	if err := out.Truncate(file.SizeBytes); err != nil {
		return err
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(chunkedMaterializeWorkers)
	for _, chunk := range file.Chunks {
		chunk := chunk
		group.Go(func() error {
			return materializeChunk(groupCtx, out, target, file, chunk, state)
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	closed = true
	if err := verifyFileDigest(target, file.Digest); err != nil {
		_ = state.clearFileMarkers(file.Path)
		return err
	}
	return nil
}

func materializeChunk(
	ctx context.Context,
	out *os.File,
	target string,
	file chunkIndexFile,
	chunk chunkIndexChunk,
	state *chunkMaterializeState,
) error {
	if state.chunkComplete(target, file.Path, chunk) {
		return nil
	}
	payload, err := state.loadStoredChunk(ctx, chunk)
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
	if err := state.markChunkComplete(file.Path, chunk); err != nil {
		return err
	}
	slog.Default().Debug(
		"oci chunked materialization chunk completed",
		slog.String("path", file.Path),
		slog.Int("chunk", chunk.Index),
		slog.Int64("offset", chunk.Offset),
		slog.Int64("sizeBytes", chunk.UncompressedSizeBytes),
	)
	return nil
}
