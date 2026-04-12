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
	"io"
	"path"
	"path/filepath"
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func rawStageEnabled(rawStage *RawStageOptions) bool {
	return rawStage != nil && rawStage.Client != nil &&
		strings.TrimSpace(rawStage.Bucket) != "" &&
		strings.TrimSpace(rawStage.KeyPrefix) != ""
}

func rawStageObjectKey(keyPrefix, relativePath string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(keyPrefix), "/")
	cleanRelative := strings.Trim(strings.ReplaceAll(strings.TrimSpace(relativePath), "\\", "/"), "/")
	switch {
	case cleanPrefix == "":
		return cleanRelative
	case cleanRelative == "":
		return cleanPrefix
	default:
		return path.Join(cleanPrefix, cleanRelative)
	}
}

func stageRawObject(
	ctx context.Context,
	rawStage RawStageOptions,
	relativePath string,
	fileName string,
	sizeBytes int64,
	contentType string,
	body io.Reader,
) (cleanuphandle.UploadStagingHandle, error) {
	if !rawStageEnabled(&rawStage) {
		return cleanuphandle.UploadStagingHandle{}, errors.New("raw stage options must not be empty")
	}
	if body == nil {
		return cleanuphandle.UploadStagingHandle{}, errors.New("raw stage body must not be nil")
	}

	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		return cleanuphandle.UploadStagingHandle{}, errors.New("raw stage relative path must not be empty")
	}
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		fileName = filepath.Base(relativePath)
	}

	handle := cleanuphandle.UploadStagingHandle{
		Bucket:    strings.TrimSpace(rawStage.Bucket),
		Key:       rawStageObjectKey(rawStage.KeyPrefix, relativePath),
		FileName:  fileName,
		SizeBytes: nonNegativeInt64(sizeBytes),
	}
	if err := rawStage.Client.Upload(ctx, uploadstagingports.UploadInput{
		Bucket:      handle.Bucket,
		Key:         handle.Key,
		Body:        body,
		ContentType: contentType,
	}); err != nil {
		return cleanuphandle.UploadStagingHandle{}, err
	}

	return handle, nil
}
func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
