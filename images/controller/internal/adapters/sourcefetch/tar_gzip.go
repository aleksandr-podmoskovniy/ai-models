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
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func newTarReader(path string, stream io.Reader) (*tar.Reader, error) {
	if strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz") {
		gzipReader, err := gzip.NewReader(stream)
		if err != nil {
			return nil, err
		}
		return tar.NewReader(gzipReader), nil
	}
	return tar.NewReader(stream), nil
}

func isTarArchive(path string) bool {
	return strings.HasSuffix(path, ".tar") || strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz")
}

func isZipArchive(path string) bool {
	return strings.HasSuffix(path, ".zip")
}

func isZipSymlink(file interface{ Mode() os.FileMode }) bool {
	return file.Mode()&os.ModeSymlink != 0
}

func writeGzipTar(buffer *bytes.Buffer, name string, content []byte) error {
	gzipWriter := gzip.NewWriter(buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{Name: filepath.ToSlash(name), Mode: 0o644, Size: int64(len(content))}
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tarWriter.Write(content); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return gzipWriter.Close()
}
