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

package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestDescribeGeneratedArchiveLayerPreservesDescriptorFields(t *testing.T) {
	t.Parallel()

	payload := []byte("synthetic archive payload\n")
	layer := modelpackports.PublishLayer{
		TargetPath:  " model/layer ",
		Base:        modelpackports.LayerBaseCode,
		Format:      modelpackports.LayerFormatTar,
		Compression: "",
	}

	got, err := describeGeneratedArchiveLayer(layer, "application/vnd.test.layer", func(writer io.Writer) error {
		_, err := writer.Write(payload)
		return err
	})
	if err != nil {
		t.Fatalf("describeGeneratedArchiveLayer() error = %v", err)
	}

	digest := testSHA256Digest(payload)
	want := publishLayerDescriptor{
		Digest:      digest,
		DiffID:      digest,
		Size:        int64(len(payload)),
		MediaType:   "application/vnd.test.layer",
		TargetPath:  "model/layer",
		Base:        modelpackports.LayerBaseCode,
		Format:      modelpackports.LayerFormatTar,
		Compression: modelpackports.LayerCompressionNone,
	}
	if got != want {
		t.Fatalf("descriptor mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCloseGeneratedArchivePipePrefersWriteError(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	reader, writer := io.Pipe()

	go closeGeneratedArchivePipe(writer, closeErrorWriter{err: closeErr}, writeErr)

	_, err := io.ReadAll(reader)
	if !errors.Is(err, writeErr) {
		t.Fatalf("ReadAll() error = %v, want write error %v", err, writeErr)
	}
	if errors.Is(err, closeErr) {
		t.Fatalf("ReadAll() error = %v, must not expose close error %v", err, closeErr)
	}
}

func TestCloseGeneratedArchivePipeReturnsCloseErrorWithoutWriteError(t *testing.T) {
	t.Parallel()

	closeErr := errors.New("close failed")
	reader, writer := io.Pipe()

	go closeGeneratedArchivePipe(writer, closeErrorWriter{err: closeErr}, nil)

	_, err := io.ReadAll(reader)
	if !errors.Is(err, closeErr) {
		t.Fatalf("ReadAll() error = %v, want close error %v", err, closeErr)
	}
}

type closeErrorWriter struct {
	err error
}

func (w closeErrorWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w closeErrorWriter) Close() error {
	return w.err
}

func testSHA256Digest(payload []byte) string {
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}
