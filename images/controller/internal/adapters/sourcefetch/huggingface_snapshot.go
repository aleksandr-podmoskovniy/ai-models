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
	"os"
	"path/filepath"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func stageHuggingFaceSnapshotFiles(
	ctx context.Context,
	snapshotDir string,
	files []string,
	rawStage RawStageOptions,
) ([]cleanuphandle.UploadStagingHandle, error) {
	if !rawStageEnabled(&rawStage) {
		return nil, nil
	}

	stagedObjects := make([]cleanuphandle.UploadStagingHandle, 0, len(files))
	for _, relativePath := range files {
		cleanPath, err := cleanRemoteRelativePath(relativePath)
		if err != nil {
			return nil, err
		}

		sourcePath := filepath.Join(snapshotDir, cleanPath)
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return nil, err
		}

		stream, err := os.Open(sourcePath)
		if err != nil {
			return nil, err
		}

		handle, stageErr := stageRawObject(
			ctx,
			rawStage,
			cleanPath,
			filepath.Base(cleanPath),
			sourceInfo.Size(),
			"",
			stream,
		)
		closeErr := stream.Close()
		if stageErr != nil {
			return nil, stageErr
		}
		if closeErr != nil {
			return nil, closeErr
		}

		stagedObjects = append(stagedObjects, handle)
	}

	return stagedObjects, nil
}

func materializeHuggingFaceSnapshot(snapshotDir, destination string, files []string) error {
	for _, relativePath := range files {
		cleanPath, err := cleanRemoteRelativePath(relativePath)
		if err != nil {
			return err
		}
		if err := linkOrCopyFile(filepath.Join(snapshotDir, cleanPath), filepath.Join(destination, cleanPath)); err != nil {
			return err
		}
	}
	return nil
}
