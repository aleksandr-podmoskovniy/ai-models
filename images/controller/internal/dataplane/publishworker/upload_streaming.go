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
	"log/slog"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
)

func tryPublishStreamingUploadArchive(
	ctx context.Context,
	options Options,
	uploadPath string,
	logger *slog.Logger,
) (publicationartifact.Result, bool, error) {
	if !isArchiveUploadPath(uploadPath) {
		return publicationartifact.Result{}, false, nil
	}

	inspection, err := sourcefetch.InspectModelArchive(uploadPath, options.InputFormat)
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	if !supportsArchiveUpload(inspection) {
		return publicationartifact.Result{}, false, nil
	}

	result, err := publishUploadArchive(ctx, options, logger, uploadArchivePublication{
		sourcePath:      uploadPath,
		artifactURI:     uploadPath,
		compressionPath: uploadPath,
		inspection:      inspection,
		logMessage:      "upload archive streaming path selected",
		logArgs:         []any{slog.String("uploadPath", uploadPath)},
	})
	if err != nil {
		return publicationartifact.Result{}, false, err
	}
	return result, true, nil
}
