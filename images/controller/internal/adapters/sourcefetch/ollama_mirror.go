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
	"fmt"
	"net/http"
	"strings"
	"time"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func prepareOllamaSourceMirror(
	ctx context.Context,
	options RemoteOptions,
	ref ollamaReference,
	files []RemoteObjectFile,
) (*SourceMirrorSnapshot, error) {
	if options.SourceMirror == nil || options.SourceMirror.Store == nil {
		return nil, nil
	}
	locator := sourcemirrorports.SnapshotLocator{
		Provider: "ollama",
		Subject:  ref.Subject,
		Revision: ref.Tag,
	}
	manifestFiles := make([]sourcemirrorports.SnapshotFile, 0, len(files))
	for _, file := range files {
		manifestFiles = append(manifestFiles, sourcemirrorports.SnapshotFile{
			Path:      file.TargetPath,
			SizeBytes: file.SizeBytes,
			ETag:      file.ETag,
		})
	}
	if err := options.SourceMirror.Store.SaveManifest(ctx, sourcemirrorports.SnapshotManifest{
		Locator:   locator,
		Files:     manifestFiles,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	return &SourceMirrorSnapshot{
		Locator:       locator,
		CleanupPrefix: sourcemirrorports.SnapshotPrefix(options.SourceMirror.BasePrefix, locator),
	}, nil
}

func persistOllamaMirrorPhase(
	ctx context.Context,
	options *SourceMirrorOptions,
	snapshot *SourceMirrorSnapshot,
	phase sourcemirrorports.SnapshotPhase,
	files []RemoteObjectFile,
	lastError string,
) error {
	if options == nil || options.Store == nil || snapshot == nil {
		return nil
	}
	stateFiles := make([]sourcemirrorports.SnapshotFileState, 0, len(files))
	for _, file := range files {
		stateFiles = append(stateFiles, sourcemirrorports.SnapshotFileState{
			Path:      file.TargetPath,
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

func transferOllamaMirrorSnapshot(
	ctx context.Context,
	options RemoteOptions,
	files []RemoteObjectFile,
	snapshot *SourceMirrorSnapshot,
) error {
	mirror := options.SourceMirror
	if mirror == nil || mirror.Store == nil || mirror.Client == nil || snapshot == nil {
		return fmt.Errorf("ollama source mirror options must be fully configured")
	}
	tracker, err := loadSourceMirrorTracker(ctx, mirror, snapshot)
	if err != nil {
		return err
	}
	if err := tracker.setSnapshotPhase(ctx, sourcemirrorports.SnapshotPhaseDownloading); err != nil {
		return err
	}
	for _, file := range files {
		if err := mirrorOllamaFile(ctx, mirror, snapshot, tracker, file); err != nil {
			_ = tracker.failFile(ctx, file.TargetPath, err)
			_ = tracker.setSnapshotPhaseWithError(ctx, sourcemirrorports.SnapshotPhaseFailed, err)
			return err
		}
	}
	if err := tracker.setSnapshotPhase(ctx, sourcemirrorports.SnapshotPhaseCompleted); err != nil {
		return err
	}
	snapshot.ObjectCount = int64(len(files))
	snapshot.SizeBytes = tracker.totalBytesConfirmed()
	return nil
}

func mirrorOllamaFile(
	ctx context.Context,
	options *SourceMirrorOptions,
	snapshot *SourceMirrorSnapshot,
	tracker *sourceMirrorTracker,
	file RemoteObjectFile,
) error {
	objectKey := sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, file.TargetPath)
	stat, err := options.Client.Stat(ctx, uploadstagingports.StatInput{Bucket: options.Bucket, Key: objectKey})
	if err == nil && stat.SizeBytes >= 0 {
		return tracker.completeFile(ctx, file.TargetPath, stat.SizeBytes)
	}

	fileState, err := tracker.ensureUpload(ctx, file.TargetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(fileState.MultipartUploadID) != "" {
		if err := tracker.syncUploadedParts(ctx, options, snapshot, file.TargetPath); err != nil {
			return err
		}
		fileState = tracker.fileState(file.TargetPath)
	}

	reader := ollamaObjectReader{httpClient: http.DefaultClient}
	opened, err := reader.OpenReadRange(ctx, file.SourcePath, fileState.BytesConfirmed, -1)
	if err != nil {
		return err
	}
	defer opened.Body.Close()
	completedParts, uploadedBytes, err := uploadMirrorResponse(
		ctx,
		options,
		tracker,
		file.TargetPath,
		objectKey,
		fileState,
		opened.Body,
	)
	if err != nil {
		return err
	}
	if len(completedParts) == 0 {
		return fmt.Errorf("ollama source mirror uploaded zero multipart parts")
	}
	if err := options.Client.CompleteMultipartUpload(ctx, uploadstagingports.CompleteMultipartUploadInput{
		Bucket:   options.Bucket,
		Key:      objectKey,
		UploadID: fileState.MultipartUploadID,
		Parts:    completedParts,
	}); err != nil {
		return err
	}
	return tracker.completeFile(ctx, file.TargetPath, uploadedBytes)
}
