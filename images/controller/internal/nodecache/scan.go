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

package nodecache

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type Entry struct {
	Digest         string
	DestinationDir string
	ModelPath      string
	MarkerPath     string
	MediaType      string
	ReadyAt        time.Time
	LastUsedAt     time.Time
	SizeBytes      int64
	Current        bool
	Ready          bool
	Failure        string
}

type Snapshot struct {
	CacheRoot      string
	CurrentTarget  string
	Entries        []Entry
	TotalSizeBytes int64
}

func Scan(cacheRoot string) (Snapshot, error) {
	cacheRoot = filepath.Clean(strings.TrimSpace(cacheRoot))
	if cacheRoot == "" || cacheRoot == "." {
		return Snapshot{}, errors.New("cache-root must not be empty")
	}

	currentTarget, err := resolveCurrentTarget(cacheRoot)
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{
		CacheRoot:     cacheRoot,
		CurrentTarget: currentTarget,
	}

	dirEntries, err := os.ReadDir(StoreRoot(cacheRoot))
	if errors.Is(err, os.ErrNotExist) {
		return snapshot, nil
	}
	if err != nil {
		return Snapshot{}, err
	}

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		entry, err := scanEntry(cacheRoot, currentTarget, dirEntry.Name())
		if err != nil {
			return Snapshot{}, err
		}
		snapshot.TotalSizeBytes += entry.SizeBytes
		snapshot.Entries = append(snapshot.Entries, entry)
	}
	sort.Slice(snapshot.Entries, func(i, j int) bool {
		return snapshot.Entries[i].Digest < snapshot.Entries[j].Digest
	})
	return snapshot, nil
}

func scanEntry(cacheRoot, currentTarget, digest string) (Entry, error) {
	destinationDir := StorePath(cacheRoot, digest)
	modelPath := modelpackports.MaterializedModelPath(destinationDir)
	entry := Entry{
		Digest:         strings.TrimSpace(digest),
		DestinationDir: destinationDir,
		ModelPath:      modelPath,
		MarkerPath:     MarkerPath(destinationDir),
	}

	sizeBytes, err := directorySize(destinationDir)
	if err != nil {
		return Entry{}, err
	}
	entry.SizeBytes = sizeBytes
	entry.Current = samePath(currentTarget, modelPath)

	marker, err := ReadMarker(destinationDir)
	if err != nil {
		entry.Failure = err.Error()
		return entry, nil
	}
	if marker != nil {
		if strings.TrimSpace(marker.Digest) != "" {
			entry.Digest = strings.TrimSpace(marker.Digest)
		}
		entry.MediaType = strings.TrimSpace(marker.MediaType)
		entry.ReadyAt = marker.ReadyAt.UTC()
		if strings.TrimSpace(marker.ModelPath) != "" {
			entry.ModelPath = filepath.Clean(strings.TrimSpace(marker.ModelPath))
		}
	}
	if _, err := os.Stat(modelPath); err != nil {
		if marker != nil {
			entry.Failure = fmt.Sprintf("model path is not ready: %v", err)
		}
		return entry, nil
	}
	if marker == nil {
		entry.Failure = "materialization marker is missing"
		return entry, nil
	}

	if lastUsed, ok, err := ReadLastUsed(destinationDir); err == nil && ok {
		entry.LastUsedAt = lastUsed
	} else if err != nil {
		entry.Failure = fmt.Sprintf("last-used marker is malformed: %v", err)
		return entry, nil
	} else if !entry.ReadyAt.IsZero() {
		entry.LastUsedAt = entry.ReadyAt
	}
	entry.Ready = true
	return entry, nil
}

func resolveCurrentTarget(cacheRoot string) (string, error) {
	target, err := os.Readlink(CurrentLinkPath(cacheRoot))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(target) {
		return filepath.Clean(target), nil
	}
	return filepath.Clean(filepath.Join(cacheRoot, target)), nil
}

func directorySize(root string) (int64, error) {
	var sizeBytes int64
	if err := filepath.WalkDir(root, func(path string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if dirEntry.IsDir() {
			return nil
		}
		info, err := dirEntry.Info()
		if err != nil {
			return err
		}
		sizeBytes += info.Size()
		return nil
	}); err != nil {
		return 0, err
	}
	return sizeBytes, nil
}

func samePath(left, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	return left != "" && left != "." && right != "" && right != "." && left == right
}
