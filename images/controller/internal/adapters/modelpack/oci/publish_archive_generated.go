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
	"io"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type generatedArchiveWriteFunc func(io.Writer) error

func describeGeneratedArchiveLayer(
	layer modelpackports.PublishLayer,
	mediaType string,
	writeArchive generatedArchiveWriteFunc,
) (publishLayerDescriptor, error) {
	diffHasher := sha256.New()
	blobHasher := sha256.New()
	counter := &countWriter{}

	archiveWriter, err := newArchiveWriter(io.MultiWriter(blobHasher, counter), layer.Compression)
	if err != nil {
		return publishLayerDescriptor{}, err
	}
	if err := writeArchive(io.MultiWriter(diffHasher, archiveWriter)); err != nil {
		_ = archiveWriter.Close()
		return publishLayerDescriptor{}, err
	}
	if err := archiveWriter.Close(); err != nil {
		return publishLayerDescriptor{}, err
	}

	return generatedArchiveLayerDescriptor(layer, mediaType, blobHasher.Sum(nil), diffHasher.Sum(nil), counter.n), nil
}

func generatedArchiveLayerDescriptor(
	layer modelpackports.PublishLayer,
	mediaType string,
	blobDigest []byte,
	diffID []byte,
	size int64,
) publishLayerDescriptor {
	return publishLayerDescriptor{
		Digest:      "sha256:" + hex.EncodeToString(blobDigest),
		DiffID:      "sha256:" + hex.EncodeToString(diffID),
		Size:        size,
		MediaType:   mediaType,
		TargetPath:  filepath.ToSlash(strings.TrimSpace(layer.TargetPath)),
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: normalizedArchiveCompression(layer.Compression),
	}
}

func openGeneratedArchiveLayerRange(
	layer modelpackports.PublishLayer,
	offset int64,
	length int64,
	writeArchive generatedArchiveWriteFunc,
) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	go func() {
		archiveWriter, openErr := newArchiveWriter(writer, layer.Compression)
		if openErr != nil {
			_ = writer.CloseWithError(openErr)
			return
		}
		closeGeneratedArchivePipe(writer, archiveWriter, writeArchive(archiveWriter))
	}()

	return rangedArchivePipeReader(reader, offset, length), nil
}

func closeGeneratedArchivePipe(writer *io.PipeWriter, archiveWriter closeWriter, writeErr error) {
	closeErr := archiveWriter.Close()
	if writeErr != nil {
		_ = writer.CloseWithError(writeErr)
		return
	}
	_ = writer.CloseWithError(closeErr)
}

func rangedArchivePipeReader(reader *io.PipeReader, offset int64, length int64) io.ReadCloser {
	stream := &archivePipeStream{reader: reader}
	body := io.Reader(stream.reader)
	if offset > 0 {
		body = &offsetReader{reader: body, offset: offset}
	}
	if length >= 0 {
		body = io.LimitReader(body, length)
	}
	return &archiveRangeReader{body: body, stream: stream}
}
