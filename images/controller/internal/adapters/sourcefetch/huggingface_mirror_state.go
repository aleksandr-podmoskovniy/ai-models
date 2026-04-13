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
	"errors"
	"strings"
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type huggingFaceMirrorTracker struct {
	options  *SourceMirrorOptions
	snapshot *SourceMirrorSnapshot
	state    sourcemirrorports.SnapshotState
}

func loadHuggingFaceMirrorTracker(
	ctx context.Context,
	options *SourceMirrorOptions,
	snapshot *SourceMirrorSnapshot,
) (*huggingFaceMirrorTracker, error) {
	state, err := options.Store.LoadState(ctx, snapshot.Locator)
	if err != nil {
		if !errors.Is(err, sourcemirrorports.ErrSnapshotNotFound) {
			return nil, err
		}
		state = sourcemirrorports.SnapshotState{
			Locator: snapshot.Locator,
			Phase:   sourcemirrorports.SnapshotPhasePending,
		}
	}
	return &huggingFaceMirrorTracker{
		options:  options,
		snapshot: snapshot,
		state:    state,
	}, nil
}

func (t *huggingFaceMirrorTracker) setSnapshotPhase(ctx context.Context, phase sourcemirrorports.SnapshotPhase) error {
	return t.setSnapshotPhaseWithError(ctx, phase, nil)
}

func (t *huggingFaceMirrorTracker) setSnapshotPhaseWithError(ctx context.Context, phase sourcemirrorports.SnapshotPhase, runErr error) error {
	t.state.Phase = phase
	t.state.UpdatedAt = time.Now().UTC()
	if runErr != nil {
		for index := range t.state.Files {
			if t.state.Files[index].Phase != sourcemirrorports.SnapshotPhaseCompleted {
				t.state.Files[index].Phase = sourcemirrorports.SnapshotPhaseFailed
				t.state.Files[index].LastError = strings.TrimSpace(runErr.Error())
				t.state.Files[index].UpdatedAt = t.state.UpdatedAt
			}
		}
	}
	return t.options.Store.SaveState(ctx, t.state)
}

func (t *huggingFaceMirrorTracker) ensureUpload(ctx context.Context, relativePath string) (sourcemirrorports.SnapshotFileState, error) {
	state := t.fileState(relativePath)
	if state.Phase == sourcemirrorports.SnapshotPhaseCompleted {
		return state, nil
	}
	if strings.TrimSpace(state.MultipartUploadID) == "" {
		started, err := t.options.Client.StartMultipartUpload(ctx, uploadstagingports.StartMultipartUploadInput{
			Bucket: t.options.Bucket,
			Key:    sourcemirrorports.SnapshotFileObjectKey(t.snapshot.CleanupPrefix, relativePath),
		})
		if err != nil {
			return sourcemirrorports.SnapshotFileState{}, err
		}
		state.MultipartUploadID = strings.TrimSpace(started.UploadID)
	}
	state.Path = relativePath
	state.Phase = sourcemirrorports.SnapshotPhaseDownloading
	state.UpdatedAt = time.Now().UTC()
	if err := t.upsertFileState(ctx, state); err != nil {
		return sourcemirrorports.SnapshotFileState{}, err
	}
	return state, nil
}

func (t *huggingFaceMirrorTracker) syncUploadedParts(
	ctx context.Context,
	options *SourceMirrorOptions,
	snapshot *SourceMirrorSnapshot,
	relativePath string,
) error {
	state := t.fileState(relativePath)
	if strings.TrimSpace(state.MultipartUploadID) == "" {
		return nil
	}
	parts, err := options.Client.ListMultipartUploadParts(ctx, uploadstagingports.ListMultipartUploadPartsInput{
		Bucket:   options.Bucket,
		Key:      sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, relativePath),
		UploadID: state.MultipartUploadID,
	})
	if err != nil {
		stat, statErr := options.Client.Stat(ctx, uploadstagingports.StatInput{
			Bucket: options.Bucket,
			Key:    sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, relativePath),
		})
		if statErr == nil {
			return t.completeFile(ctx, relativePath, stat.SizeBytes)
		}
		return err
	}
	state.CompletedParts = make([]uploadstagingports.CompletedPart, 0, len(parts))
	state.BytesConfirmed = 0
	for _, part := range parts {
		state.CompletedParts = append(state.CompletedParts, uploadstagingports.CompletedPart{
			PartNumber: part.PartNumber,
			ETag:       part.ETag,
		})
		state.BytesConfirmed += part.SizeBytes
	}
	state.Phase = sourcemirrorports.SnapshotPhaseDownloading
	state.UpdatedAt = time.Now().UTC()
	return t.upsertFileState(ctx, state)
}

func (t *huggingFaceMirrorTracker) appendCompletedPart(
	ctx context.Context,
	relativePath string,
	part uploadstagingports.CompletedPart,
	sizeBytes int64,
) error {
	state := t.fileState(relativePath)
	state.Path = relativePath
	state.Phase = sourcemirrorports.SnapshotPhaseDownloading
	state.CompletedParts = append(state.CompletedParts, part)
	state.BytesConfirmed += sizeBytes
	state.LastError = ""
	state.UpdatedAt = time.Now().UTC()
	return t.upsertFileState(ctx, state)
}

func (t *huggingFaceMirrorTracker) completeFile(ctx context.Context, relativePath string, sizeBytes int64) error {
	state := t.fileState(relativePath)
	state.Path = relativePath
	state.Phase = sourcemirrorports.SnapshotPhaseCompleted
	state.BytesConfirmed = sizeBytes
	state.LastError = ""
	state.UpdatedAt = time.Now().UTC()
	return t.upsertFileState(ctx, state)
}

func (t *huggingFaceMirrorTracker) failFile(ctx context.Context, relativePath string, runErr error) error {
	state := t.fileState(relativePath)
	state.Path = relativePath
	state.Phase = sourcemirrorports.SnapshotPhaseFailed
	state.LastError = strings.TrimSpace(runErr.Error())
	state.UpdatedAt = time.Now().UTC()
	return t.upsertFileState(ctx, state)
}

func (t *huggingFaceMirrorTracker) upsertFileState(ctx context.Context, state sourcemirrorports.SnapshotFileState) error {
	found := false
	for index := range t.state.Files {
		if t.state.Files[index].Path == state.Path {
			t.state.Files[index] = state
			found = true
			break
		}
	}
	if !found {
		t.state.Files = append(t.state.Files, state)
	}
	t.state.UpdatedAt = time.Now().UTC()
	return t.options.Store.SaveState(ctx, t.state)
}

func (t *huggingFaceMirrorTracker) fileState(relativePath string) sourcemirrorports.SnapshotFileState {
	for _, file := range t.state.Files {
		if file.Path == relativePath {
			return file
		}
	}
	return sourcemirrorports.SnapshotFileState{Path: relativePath}
}

func (t *huggingFaceMirrorTracker) totalBytesConfirmed() int64 {
	var total int64
	for _, file := range t.state.Files {
		total += file.BytesConfirmed
	}
	return total
}
