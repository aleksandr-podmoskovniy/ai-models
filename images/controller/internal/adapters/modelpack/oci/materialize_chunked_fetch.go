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
)

func (s *chunkMaterializeState) loadStoredChunk(ctx context.Context, chunk chunkIndexChunk) ([]byte, error) {
	pack, found := s.packs[chunk.Pack]
	if !found {
		return nil, fmt.Errorf("chunk %d references missing pack %q", chunk.Index, chunk.Pack)
	}
	lock := s.packLocks[pack.ID]
	lock.Lock()

	if payload, found, err := s.loadChunkFromFallbackPack(pack, chunk); err != nil || found {
		lock.Unlock()
		return payload, err
	}
	mode := s.packModes[pack.ID]
	if mode == blobRangePartial {
		lock.Unlock()
		return s.loadRemoteRangeChunk(ctx, pack, chunk)
	}
	if mode == blobRangeFullBody {
		payload, err := s.fetchAndStoreFallbackPackLocked(ctx, pack)
		lock.Unlock()
		if err != nil {
			return nil, err
		}
		return s.sliceStoredChunk(payload, pack, chunk)
	}

	payload, mode, err := FetchBlobRange(ctx, s.client, s.reference, pack.Digest, s.auth, chunk.PackOffset, chunk.PackLength)
	if err != nil {
		lock.Unlock()
		return nil, fmt.Errorf("failed to fetch ModelPack chunk %d from pack %q: %w", chunk.Index, pack.ID, err)
	}
	if mode == blobRangeFullBody {
		if err := verifyChunkDigest(payload, pack.Digest); err != nil {
			lock.Unlock()
			return nil, fmt.Errorf("chunk pack %q digest mismatch after full-body range fallback: %w", pack.ID, err)
		}
		if err := s.writeFallbackPack(pack, payload); err != nil {
			lock.Unlock()
			return nil, err
		}
		s.packModes[pack.ID] = blobRangeFullBody
		slog.Default().Info(
			"oci chunked materialization fell back to full pack download",
			slog.String("pack", pack.ID),
			slog.String("digest", pack.Digest),
			slog.Int64("sizeBytes", pack.SizeBytes),
		)
		lock.Unlock()
		return s.sliceStoredChunk(payload, pack, chunk)
	}
	s.packModes[pack.ID] = blobRangePartial
	lock.Unlock()
	if err := verifyChunkDigest(payload, chunk.StoredDigest); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *chunkMaterializeState) loadRemoteRangeChunk(ctx context.Context, pack chunkIndexPack, chunk chunkIndexChunk) ([]byte, error) {
	payload, mode, err := FetchBlobRange(ctx, s.client, s.reference, pack.Digest, s.auth, chunk.PackOffset, chunk.PackLength)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ModelPack chunk %d from pack %q: %w", chunk.Index, pack.ID, err)
	}
	if mode != blobRangePartial {
		return nil, fmt.Errorf("remote blob range mode changed for pack %q: expected %q, got %q", pack.ID, blobRangePartial, mode)
	}
	if err := verifyChunkDigest(payload, chunk.StoredDigest); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *chunkMaterializeState) fetchAndStoreFallbackPackLocked(ctx context.Context, pack chunkIndexPack) ([]byte, error) {
	payload, mode, err := FetchBlobRange(ctx, s.client, s.reference, pack.Digest, s.auth, 0, pack.SizeBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ModelPack fallback pack %q: %w", pack.ID, err)
	}
	if mode != blobRangeFullBody {
		return nil, fmt.Errorf("remote blob range mode changed for pack %q: expected %q, got %q", pack.ID, blobRangeFullBody, mode)
	}
	if err := verifyChunkDigest(payload, pack.Digest); err != nil {
		return nil, fmt.Errorf("chunk pack %q digest mismatch after full-body range fallback: %w", pack.ID, err)
	}
	if err := s.writeFallbackPack(pack, payload); err != nil {
		return nil, err
	}
	return payload, nil
}
