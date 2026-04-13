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

package objectstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/aws/smithy-go"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

const (
	manifestFileName = "manifest.json"
	stateFileName    = "state.json"
)

type Adapter struct {
	Uploader      uploadstagingports.Uploader
	Downloader    uploadstagingports.Downloader
	PrefixRemover uploadstagingports.PrefixRemover
	Bucket        string
	BasePrefix    string
}

func (a *Adapter) SaveManifest(ctx context.Context, manifest sourcemirrorports.SnapshotManifest) error {
	if err := validateAdapter(a); err != nil {
		return err
	}
	if err := manifest.Validate(); err != nil {
		return err
	}
	return a.uploadJSON(ctx, manifestObjectKey(a.BasePrefix, manifest.Locator), manifest)
}

func (a *Adapter) LoadManifest(ctx context.Context, locator sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotManifest, error) {
	if err := validateAdapter(a); err != nil {
		return sourcemirrorports.SnapshotManifest{}, err
	}
	if err := locator.Validate(); err != nil {
		return sourcemirrorports.SnapshotManifest{}, err
	}
	var manifest sourcemirrorports.SnapshotManifest
	if err := a.downloadJSON(ctx, manifestObjectKey(a.BasePrefix, locator), &manifest); err != nil {
		return sourcemirrorports.SnapshotManifest{}, err
	}
	return manifest, nil
}

func (a *Adapter) SaveState(ctx context.Context, state sourcemirrorports.SnapshotState) error {
	if err := validateAdapter(a); err != nil {
		return err
	}
	if err := state.Validate(); err != nil {
		return err
	}
	return a.uploadJSON(ctx, stateObjectKey(a.BasePrefix, state.Locator), state)
}

func (a *Adapter) LoadState(ctx context.Context, locator sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotState, error) {
	if err := validateAdapter(a); err != nil {
		return sourcemirrorports.SnapshotState{}, err
	}
	if err := locator.Validate(); err != nil {
		return sourcemirrorports.SnapshotState{}, err
	}
	var state sourcemirrorports.SnapshotState
	if err := a.downloadJSON(ctx, stateObjectKey(a.BasePrefix, locator), &state); err != nil {
		return sourcemirrorports.SnapshotState{}, err
	}
	return state, nil
}

func (a *Adapter) DeleteSnapshot(ctx context.Context, locator sourcemirrorports.SnapshotLocator) error {
	if err := validateAdapter(a); err != nil {
		return err
	}
	if a.PrefixRemover == nil {
		return errors.New("source mirror prefix remover must not be nil")
	}
	if err := locator.Validate(); err != nil {
		return err
	}
	return a.PrefixRemover.DeletePrefix(ctx, uploadstagingports.DeletePrefixInput{
		Bucket: strings.TrimSpace(a.Bucket),
		Prefix: snapshotPrefix(a.BasePrefix, locator),
	})
}

func (a *Adapter) uploadJSON(ctx context.Context, key string, payload any) error {
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return a.Uploader.Upload(ctx, uploadstagingports.UploadInput{
		Bucket:      strings.TrimSpace(a.Bucket),
		Key:         key,
		Body:        bytes.NewReader(body),
		ContentType: "application/json",
	})
}

func (a *Adapter) downloadJSON(ctx context.Context, key string, into any) error {
	tempFile, err := os.CreateTemp("", "ai-model-source-mirror-*.json")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	defer os.Remove(tempPath)

	if err := a.Downloader.Download(ctx, uploadstagingports.DownloadInput{
		Bucket:          strings.TrimSpace(a.Bucket),
		Key:             key,
		DestinationPath: tempPath,
	}); err != nil {
		if isNotFoundError(err) {
			return sourcemirrorports.ErrSnapshotNotFound
		}
		return err
	}

	body, err := os.ReadFile(tempPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("failed to decode source mirror JSON %q: %w", key, err)
	}
	return nil
}

func validateAdapter(a *Adapter) error {
	switch {
	case a == nil:
		return errors.New("source mirror adapter must not be nil")
	case a.Uploader == nil:
		return errors.New("source mirror uploader must not be nil")
	case a.Downloader == nil:
		return errors.New("source mirror downloader must not be nil")
	case strings.TrimSpace(a.Bucket) == "":
		return errors.New("source mirror bucket must not be empty")
	case strings.Trim(strings.TrimSpace(a.BasePrefix), "/") == "":
		return errors.New("source mirror base prefix must not be empty")
	default:
		return nil
	}
}

func snapshotPrefix(basePrefix string, locator sourcemirrorports.SnapshotLocator) string {
	return sourcemirrorports.SnapshotPrefix(basePrefix, locator)
}

func manifestObjectKey(basePrefix string, locator sourcemirrorports.SnapshotLocator) string {
	return path.Join(snapshotPrefix(basePrefix, locator), manifestFileName)
}

func stateObjectKey(basePrefix string, locator sourcemirrorports.SnapshotLocator) string {
	return path.Join(snapshotPrefix(basePrefix, locator), stateFileName)
}

func isNotFoundError(err error) bool {
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		code := strings.TrimSpace(apiError.ErrorCode())
		return code == "NoSuchKey" || code == "NotFound"
	}
	return false
}
