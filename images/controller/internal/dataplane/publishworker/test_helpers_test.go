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
	"archive/tar"
	"archive/zip"
	"context"
	"os"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type fakePublisher struct {
	onPublish func(modelpackports.PublishInput) error
}

func (f fakePublisher) Publish(_ context.Context, input modelpackports.PublishInput, _ modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	if f.onPublish != nil {
		if err := f.onPublish(input); err != nil {
			return modelpackports.PublishResult{}, err
		}
	}
	return modelpackports.PublishResult{
		Reference: "registry.example.com/ai-models/catalog/model@sha256:deadbeef",
		Digest:    "sha256:deadbeef",
		MediaType: "application/vnd.cncf.model.manifest.v1+json",
		SizeBytes: 123,
	}, nil
}

type tarEntry struct {
	name    string
	content []byte
}

func createTestTar(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := tar.NewWriter(file)
	defer writer.Close()

	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0o644, Size: int64(len(entry.content))}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if _, err := writer.Write(entry.content); err != nil {
			return err
		}
	}
	return nil
}

func createTestZip(path string, entries ...tarEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	for _, entry := range entries {
		stream, err := writer.Create(entry.name)
		if err != nil {
			return err
		}
		if _, err := stream.Write(entry.content); err != nil {
			return err
		}
	}
	return nil
}

func writeTempFile(t *testing.T, name string, payload []byte) string {
	t.Helper()

	path := t.TempDir() + "/" + name
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
