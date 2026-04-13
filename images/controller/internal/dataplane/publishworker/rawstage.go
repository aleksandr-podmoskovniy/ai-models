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

package publishworker

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	sourcemirrorobjectstore "github.com/deckhouse/ai-models/controller/internal/adapters/sourcemirror/objectstore"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type uploadHTTPClientProvider interface {
	HTTPClient() *http.Client
}

func remoteRawStage(options Options) *sourcefetch.RawStageOptions {
	if strings.TrimSpace(options.RawStageBucket) == "" || strings.TrimSpace(options.RawStageKeyPrefix) == "" {
		return nil
	}
	if options.UploadStaging == nil {
		return nil
	}
	return &sourcefetch.RawStageOptions{
		Bucket:    options.RawStageBucket,
		KeyPrefix: options.RawStageKeyPrefix,
		Client:    options.UploadStaging,
	}
}

func cleanupRemoteStagedObjects(
	ctx context.Context,
	options Options,
	objects []cleanuphandle.UploadStagingHandle,
) error {
	if len(objects) == 0 {
		return nil
	}
	if options.UploadStaging == nil {
		return errors.New("upload staging client must not be nil")
	}
	for _, object := range objects {
		if err := options.UploadStaging.Delete(ctx, uploadstagingports.DeleteInput{
			Bucket: object.Bucket,
			Key:    object.Key,
		}); err != nil {
			return err
		}
	}
	return nil
}

func remoteSourceMirror(options Options) *sourcefetch.SourceMirrorOptions {
	if strings.TrimSpace(options.RawStageBucket) == "" || strings.TrimSpace(options.RawStageKeyPrefix) == "" {
		return nil
	}
	if options.UploadStaging == nil {
		return nil
	}
	return &sourcefetch.SourceMirrorOptions{
		Bucket:           strings.TrimSpace(options.RawStageBucket),
		Client:           options.UploadStaging,
		UploadHTTPClient: uploadStagingHTTPClient(options.UploadStaging),
		Store: &sourcemirrorobjectstore.Adapter{
			Uploader:   options.UploadStaging,
			Downloader: options.UploadStaging,
			Bucket:     options.RawStageBucket,
			BasePrefix: path.Join(options.RawStageKeyPrefix, ".mirror"),
		},
		BasePrefix: path.Join(options.RawStageKeyPrefix, ".mirror"),
	}
}

func uploadStagingHTTPClient(client uploadstagingports.Client) *http.Client {
	provider, ok := client.(uploadHTTPClientProvider)
	if !ok {
		return nil
	}
	return provider.HTTPClient()
}
