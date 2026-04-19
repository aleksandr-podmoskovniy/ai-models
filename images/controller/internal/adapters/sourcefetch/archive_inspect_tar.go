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
	"archive/tar"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

func listTarArchiveFilesFromReader(path string, stream io.Reader) ([]tarArchiveFile, string, error) {
	reader, closeArchive, err := newClosableTarReader(path, stream)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = closeArchive() }()

	files := make([]tarArchiveFile, 0, 16)
	meaningfulRoots := make(map[string]struct{}, 2)
	hasArchiveRootFile := false
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", err
		}

		relative, keep, err := classifyArchiveEntry(header)
		if err != nil {
			return nil, "", err
		}
		if !keep {
			continue
		}
		files = append(files, tarArchiveFile{RelativePath: relative, SizeBytes: header.Size})

		root := archiveTopLevelRoot(relative)
		if root == "" {
			hasArchiveRootFile = true
			continue
		}
		meaningfulRoots[root] = struct{}{}
	}

	return files, deriveArchiveRootPrefix(meaningfulRoots, hasArchiveRootFile), nil
}

func classifyArchiveEntry(header *tar.Header) (string, bool, error) {
	relative, err := archiveRelativePath(header.Name)
	if err != nil {
		return "", false, err
	}
	switch header.Typeflag {
	case tar.TypeDir, tar.TypeXHeader, tar.TypeXGlobalHeader:
		return relative, false, nil
	case tar.TypeReg, tar.TypeRegA:
		return relative, true, nil
	case tar.TypeSymlink:
		return "", false, errors.New("refusing to inspect symbolic link tar entry")
	case tar.TypeLink:
		return "", false, errors.New("refusing to inspect hard link tar entry")
	default:
		return "", false, errors.New("refusing to inspect unsupported tar entry")
	}
}

func archiveTopLevelRoot(relative string) string {
	clean := strings.TrimSpace(filepath.ToSlash(relative))
	if clean == "" || clean == "." {
		return ""
	}
	parts := strings.Split(clean, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func deriveArchiveRootPrefix(roots map[string]struct{}, hasArchiveRootFile bool) string {
	if hasArchiveRootFile || len(roots) != 1 {
		return ""
	}
	for root := range roots {
		return strings.TrimSpace(root)
	}
	return ""
}

func normalizedArchiveFilePath(relativePath string, rootPrefix string) (string, bool) {
	trimmed := strings.TrimSpace(filepath.ToSlash(relativePath))
	if trimmed == "" || trimmed == "." {
		return "", false
	}
	if rootPrefix == "" {
		return trimmed, true
	}
	prefix := strings.Trim(strings.TrimSpace(rootPrefix), "/") + "/"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimPrefix(trimmed, prefix), true
	}
	return trimmed, true
}
