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
	"encoding/json"
	"errors"
	"io"
)

const materializedLayerPath = "model"

type countWriter struct {
	n int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += int64(len(p))
	return len(p), nil
}

type offsetReader struct {
	reader  io.Reader
	offset  int64
	skipped bool
}

func (r *offsetReader) Read(p []byte) (int, error) {
	if !r.skipped {
		if r.offset > 0 {
			if _, err := io.CopyN(io.Discard, r.reader, r.offset); err != nil {
				return 0, err
			}
		}
		r.skipped = true
	}
	return r.reader.Read(p)
}

type archivePipeStream struct {
	reader *io.PipeReader
}

type archiveRangeReader struct {
	body   io.Reader
	stream *archivePipeStream
}

func (r *archiveRangeReader) Read(p []byte) (int, error) {
	return r.body.Read(p)
}

func (r *archiveRangeReader) Close() error {
	closeErr := r.stream.reader.Close()
	if errors.Is(closeErr, io.ErrClosedPipe) {
		return nil
	}
	return closeErr
}

func newBlobDescriptor(payload []byte) (blobDescriptor, error) {
	if len(payload) == 0 {
		return blobDescriptor{}, errors.New("blob payload must not be empty")
	}
	digestBytes := sha256.Sum256(payload)
	return blobDescriptor{
		Digest: "sha256:" + hex.EncodeToString(digestBytes[:]),
		Size:   int64(len(payload)),
	}, nil
}

func jsonMarshal(value any) ([]byte, error) {
	return json.Marshal(value)
}
