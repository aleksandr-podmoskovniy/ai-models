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
	"archive/zip"
	"errors"
	"io"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func InspectZipModelArchiveReaderAt(
	path string,
	sizeBytes int64,
	readerAt io.ReaderAt,
	requested modelsv1alpha1.ModelInputFormat,
) (ArchiveInspection, error) {
	switch {
	case !isZipArchive(path):
		return ArchiveInspection{}, errors.New("streaming zip archive inspection only supports .zip")
	case sizeBytes <= 0:
		return ArchiveInspection{}, errors.New("streaming zip archive inspection requires positive archive size")
	case readerAt == nil:
		return ArchiveInspection{}, errors.New("streaming zip archive inspection requires archive reader")
	}

	archive, err := zip.NewReader(readerAt, sizeBytes)
	if err != nil {
		return ArchiveInspection{}, err
	}
	return inspectZipArchiveFiles(archive.File, requested)
}
