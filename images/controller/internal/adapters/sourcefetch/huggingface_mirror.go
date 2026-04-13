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

package sourcefetch

import (
	"context"
	"strings"
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

func persistHuggingFaceMirrorManifest(
	ctx context.Context,
	options *SourceMirrorOptions,
	repoID string,
	resolvedRevision string,
	files []string,
) (*SourceMirrorSnapshot, error) {
	if options == nil || options.Store == nil {
		return nil, nil
	}

	locator := sourcemirrorports.SnapshotLocator{
		Provider: "huggingface",
		Subject:  strings.Trim(strings.TrimSpace(repoID), "/"),
		Revision: strings.TrimSpace(resolvedRevision),
	}
	manifestFiles := make([]sourcemirrorports.SnapshotFile, 0, len(files))
	for _, filePath := range files {
		cleanPath, err := cleanRemoteRelativePath(filePath)
		if err != nil {
			return nil, err
		}
		manifestFiles = append(manifestFiles, sourcemirrorports.SnapshotFile{Path: cleanPath})
	}
	if err := options.Store.SaveManifest(ctx, sourcemirrorports.SnapshotManifest{
		Locator:   locator,
		Files:     manifestFiles,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}

	return &SourceMirrorSnapshot{
		Locator:       locator,
		CleanupPrefix: sourcemirrorports.SnapshotPrefix(options.BasePrefix, locator),
	}, nil
}

func persistHuggingFaceMirrorPhase(
	ctx context.Context,
	options *SourceMirrorOptions,
	snapshot *SourceMirrorSnapshot,
	phase sourcemirrorports.SnapshotPhase,
	files []string,
	lastError string,
) error {
	if options == nil || options.Store == nil || snapshot == nil {
		return nil
	}

	stateFiles := make([]sourcemirrorports.SnapshotFileState, 0, len(files))
	for _, filePath := range files {
		cleanPath, err := cleanRemoteRelativePath(filePath)
		if err != nil {
			return err
		}
		stateFiles = append(stateFiles, sourcemirrorports.SnapshotFileState{
			Path:      cleanPath,
			Phase:     phase,
			LastError: strings.TrimSpace(lastError),
			UpdatedAt: time.Now().UTC(),
		})
	}

	return options.Store.SaveState(ctx, sourcemirrorports.SnapshotState{
		Locator:   snapshot.Locator,
		Phase:     phase,
		Files:     stateFiles,
		UpdatedAt: time.Now().UTC(),
	})
}
