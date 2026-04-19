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
	"net/http"
	"path"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	sourcemirrorobjectstore "github.com/deckhouse/ai-models/controller/internal/adapters/sourcemirror/objectstore"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

type uploadHTTPClientProvider interface {
	HTTPClient() *http.Client
}

func remoteSourceMirror(options Options) *sourcefetch.SourceMirrorOptions {
	if publicationports.NormalizeHuggingFaceAcquisitionMode(options.HuggingFaceAcquisitionMode) != publicationports.HuggingFaceAcquisitionModeMirror {
		return nil
	}
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
			Uploader:      options.UploadStaging,
			Reader:        options.UploadStaging,
			PrefixRemover: options.UploadStaging,
			Bucket:        options.RawStageBucket,
			BasePrefix:    path.Join(options.RawStageKeyPrefix, ".mirror"),
		},
		BasePrefix: path.Join(options.RawStageKeyPrefix, ".mirror"),
	}
}

func uploadStagingHTTPClient(client any) *http.Client {
	provider, ok := client.(uploadHTTPClientProvider)
	if !ok {
		return nil
	}
	return provider.HTTPClient()
}
