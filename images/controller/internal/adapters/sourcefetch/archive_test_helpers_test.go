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
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
)

func createGzipTar(path, name string, content []byte) error {
	buffer := &bytes.Buffer{}
	if err := writeGzipTar(buffer, name, content); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

type tarEntry struct {
	name    string
	content []byte
}

func createTarArchive(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := tar.NewWriter(file)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	return writer.Close()
}

func createGzipTarArchive(path string, entries ...tarEntry) error {
	buffer := &bytes.Buffer{}
	gzipWriter := gzip.NewWriter(buffer)
	writer := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buffer.Bytes(), 0o644)
}

func createZipArchive(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	for _, entry := range entries {
		stream, err := writer.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := stream.Write(entry.content); err != nil {
			return err
		}
	}
	return writer.Close()
}
