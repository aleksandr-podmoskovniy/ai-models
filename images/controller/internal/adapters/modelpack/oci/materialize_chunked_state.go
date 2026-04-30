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
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	chunkStateDirName      = ".ai-models-chunked"
	chunkStateVersion      = "v1"
	chunkWorkRootSuffix    = ".chunked-in-progress"
	chunkFallbackPacksDir  = "packs"
	chunkCompletionDirName = "chunks"
)

type chunkMaterializeState struct {
	client    *http.Client
	reference string
	auth      modelpackports.RegistryAuth
	stateDir  string
	packs     map[string]chunkIndexPack
	packLocks map[string]*sync.Mutex
	packModes map[string]blobRangeMode
}

type chunkRootMarker struct {
	Version        string `json:"version"`
	ArtifactDigest string `json:"artifactDigest"`
}

type chunkCompletionMarker struct {
	Version               string `json:"version"`
	Path                  string `json:"path"`
	Index                 int    `json:"index"`
	Offset                int64  `json:"offset"`
	UncompressedSizeBytes int64  `json:"uncompressedSizeBytes"`
	StoredDigest          string `json:"storedDigest"`
}

func newChunkMaterializeState(
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	destination string,
	packs []chunkIndexPack,
) *chunkMaterializeState {
	result := &chunkMaterializeState{
		client:    client,
		reference: reference,
		auth:      auth,
		stateDir:  chunkStateDir(destination),
		packs:     make(map[string]chunkIndexPack, len(packs)),
		packLocks: make(map[string]*sync.Mutex, len(packs)),
		packModes: make(map[string]blobRangeMode, len(packs)),
	}
	for _, pack := range packs {
		result.packs[pack.ID] = pack
		result.packLocks[pack.ID] = &sync.Mutex{}
	}
	return result
}

func payloadUsesChunkedLayout(payload InspectPayload) bool {
	manifest, _ := payload["manifest"].(map[string]any)
	layers, _ := manifest["layers"].([]any)
	layerSet, err := classifyManifestLayers(layers)
	return err == nil && layerSet.Chunked()
}

func chunkMaterializeWorkRoot(destination string) string {
	return strings.TrimSpace(destination) + chunkWorkRootSuffix
}

func chunkStateDir(root string) string {
	return filepath.Join(root, chunkStateDirName)
}

func prepareChunkMaterializeWorkRoot(root, digest string) error {
	markerPath := filepath.Join(chunkStateDir(root), "root.json")
	marker, err := readChunkRootMarker(markerPath)
	if err != nil {
		return err
	}
	if marker != nil && marker.ArtifactDigest != strings.TrimSpace(digest) {
		if err := os.RemoveAll(root); err != nil {
			return err
		}
		marker = nil
	}
	if err := os.MkdirAll(chunkStateDir(root), 0o755); err != nil {
		return err
	}
	if marker != nil {
		return nil
	}
	payload, err := json.MarshalIndent(chunkRootMarker{
		Version:        chunkStateVersion,
		ArtifactDigest: strings.TrimSpace(digest),
	}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(markerPath, append(payload, '\n'))
}

func readChunkRootMarker(path string) (*chunkRootMarker, error) {
	payload, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var marker chunkRootMarker
	if err := json.Unmarshal(payload, &marker); err != nil {
		return nil, fmt.Errorf("decode chunked materialize root marker: %w", err)
	}
	if marker.Version != chunkStateVersion || strings.TrimSpace(marker.ArtifactDigest) == "" {
		return nil, fmt.Errorf("invalid chunked materialize root marker %q", path)
	}
	return &marker, nil
}

func removeChunkMaterializeState(root string) error {
	return os.RemoveAll(chunkStateDir(root))
}

func (s *chunkMaterializeState) chunkComplete(target, filePath string, chunk chunkIndexChunk) bool {
	if chunk.Offset > maxInt64-chunk.UncompressedSizeBytes {
		return false
	}
	info, err := os.Stat(target)
	if err != nil || info.Size() < chunk.Offset+chunk.UncompressedSizeBytes {
		return false
	}
	marker, err := s.readChunkMarker(filePath, chunk)
	if err != nil {
		return false
	}
	return marker != nil &&
		marker.Path == filePath &&
		marker.Index == chunk.Index &&
		marker.Offset == chunk.Offset &&
		marker.UncompressedSizeBytes == chunk.UncompressedSizeBytes &&
		marker.StoredDigest == strings.TrimSpace(chunk.StoredDigest)
}

func (s *chunkMaterializeState) readChunkMarker(filePath string, chunk chunkIndexChunk) (*chunkCompletionMarker, error) {
	payload, err := os.ReadFile(s.chunkMarkerPath(filePath, chunk.Index))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var marker chunkCompletionMarker
	if err := json.Unmarshal(payload, &marker); err != nil {
		return nil, err
	}
	if marker.Version != chunkStateVersion {
		return nil, nil
	}
	return &marker, nil
}

func (s *chunkMaterializeState) markChunkComplete(filePath string, chunk chunkIndexChunk) error {
	markerPath := s.chunkMarkerPath(filePath, chunk.Index)
	payload, err := json.MarshalIndent(chunkCompletionMarker{
		Version:               chunkStateVersion,
		Path:                  filePath,
		Index:                 chunk.Index,
		Offset:                chunk.Offset,
		UncompressedSizeBytes: chunk.UncompressedSizeBytes,
		StoredDigest:          strings.TrimSpace(chunk.StoredDigest),
	}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(markerPath, append(payload, '\n'))
}

func (s *chunkMaterializeState) clearFileMarkers(filePath string) error {
	return os.RemoveAll(filepath.Join(s.stateDir, chunkCompletionDirName, digestPath(filePath)))
}

func (s *chunkMaterializeState) chunkMarkerPath(filePath string, index int) string {
	return filepath.Join(s.stateDir, chunkCompletionDirName, digestPath(filePath), fmt.Sprintf("%06d.json", index))
}

func (s *chunkMaterializeState) fallbackPackPath(pack chunkIndexPack) string {
	return filepath.Join(s.stateDir, chunkFallbackPacksDir, digestPath(pack.ID)+"-"+digestPath(pack.Digest)+".pack")
}

func (s *chunkMaterializeState) loadChunkFromFallbackPack(pack chunkIndexPack, chunk chunkIndexChunk) ([]byte, bool, error) {
	path := s.fallbackPackPath(pack)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	payload := make([]byte, chunk.PackLength)
	if _, err := file.ReadAt(payload, chunk.PackOffset); err != nil {
		return nil, false, err
	}
	if err := verifyChunkDigest(payload, chunk.StoredDigest); err != nil {
		return nil, false, err
	}
	return payload, true, nil
}

func (s *chunkMaterializeState) writeFallbackPack(pack chunkIndexPack, payload []byte) error {
	if int64(len(payload)) != pack.SizeBytes {
		return fmt.Errorf("chunk pack %q full-body fallback size %d does not match expected %d", pack.ID, len(payload), pack.SizeBytes)
	}
	return writeFileAtomic(s.fallbackPackPath(pack), payload)
}

func (s *chunkMaterializeState) sliceStoredChunk(payload []byte, pack chunkIndexPack, chunk chunkIndexChunk) ([]byte, error) {
	if chunk.PackOffset > maxInt64-chunk.PackLength {
		return nil, fmt.Errorf("chunk %d pack range overflows int64", chunk.Index)
	}
	end := chunk.PackOffset + chunk.PackLength
	if chunk.PackOffset < 0 || end > int64(len(payload)) {
		return nil, fmt.Errorf("chunk %d pack range is outside fetched pack %q", chunk.Index, pack.ID)
	}
	stored := append([]byte(nil), payload[int(chunk.PackOffset):int(end)]...)
	if err := verifyChunkDigest(stored, chunk.StoredDigest); err != nil {
		return nil, err
	}
	return stored, nil
}

func writeFileAtomic(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func digestPath(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
