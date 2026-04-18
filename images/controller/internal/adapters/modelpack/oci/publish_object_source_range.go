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
	"archive/tar"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type objectSourceArchiveSegment struct {
	header []byte
	file   *modelpackports.PublishObjectFile
	pad    int64
}

func openRangedObjectSourceLayer(
	ctx context.Context,
	layer modelpackports.PublishLayer,
	reader modelpackports.PublishObjectRangeReader,
	offset, length int64,
) (io.ReadCloser, error) {
	segments, err := buildObjectSourceArchiveSegments(layer)
	if err != nil {
		return nil, err
	}

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		writeErr := writeRangedObjectSourceArchive(ctx, pipeWriter, reader, segments, offset, length)
		_ = pipeWriter.CloseWithError(writeErr)
	}()
	return pipeReader, nil
}

func buildObjectSourceArchiveSegments(layer modelpackports.PublishLayer) ([]objectSourceArchiveSegment, error) {
	segments := make([]objectSourceArchiveSegment, 0, len(layer.ObjectSource.Files)+1)
	for _, file := range layer.ObjectSource.Files {
		targetPath, err := archiveRelativePath(file.TargetPath)
		if err != nil {
			return nil, err
		}
		header, err := tarHeaderBytes(filepath.ToSlash(strings.Trim(strings.TrimSpace(layer.TargetPath), "/")+"/"+strings.Trim(strings.TrimSpace(targetPath), "/")), file.SizeBytes)
		if err != nil {
			return nil, err
		}
		segments = append(segments, objectSourceArchiveSegment{
			header: header,
			file:   &modelpackports.PublishObjectFile{SourcePath: file.SourcePath, TargetPath: file.TargetPath, SizeBytes: file.SizeBytes, ETag: file.ETag},
			pad:    tarPaddingSize(file.SizeBytes),
		})
	}
	segments = append(segments, objectSourceArchiveSegment{pad: 1024})
	return segments, nil
}

func tarHeaderBytes(name string, size int64) ([]byte, error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	if err := writer.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: size,
	}); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func tarPaddingSize(size int64) int64 {
	if size <= 0 {
		return 0
	}
	remainder := size % 512
	if remainder == 0 {
		return 0
	}
	return 512 - remainder
}

func writeRangedObjectSourceArchive(
	ctx context.Context,
	writer *io.PipeWriter,
	rangeReader modelpackports.PublishObjectRangeReader,
	segments []objectSourceArchiveSegment,
	offset, length int64,
) error {
	currentOffset := int64(0)
	remaining := length
	unbounded := length < 0

	for _, segment := range segments {
		if len(segment.header) > 0 {
			written, err := writeSegmentBytes(writer, segment.header, currentOffset, offset, &remaining, unbounded)
			if err != nil {
				return err
			}
			currentOffset += written
		}
		if segment.file != nil {
			written, err := writeSegmentFile(ctx, writer, rangeReader, *segment.file, currentOffset, offset, &remaining, unbounded)
			if err != nil {
				return err
			}
			currentOffset += written
		}
		if segment.pad > 0 {
			written, err := writeZeroSegment(writer, segment.pad, currentOffset, offset, &remaining, unbounded)
			if err != nil {
				return err
			}
			currentOffset += written
		}
		if !unbounded && remaining <= 0 {
			return nil
		}
	}
	return nil
}

func writeSegmentBytes(
	writer io.Writer,
	segment []byte,
	currentOffset int64,
	startOffset int64,
	remaining *int64,
	unbounded bool,
) (int64, error) {
	return writeSegmentReader(writer, bytes.NewReader(segment), int64(len(segment)), currentOffset, startOffset, remaining, unbounded)
}

func writeZeroSegment(
	writer io.Writer,
	length int64,
	currentOffset int64,
	startOffset int64,
	remaining *int64,
	unbounded bool,
) (int64, error) {
	return writeSegmentReader(writer, io.LimitReader(zeroReader{}, length), length, currentOffset, startOffset, remaining, unbounded)
}

func writeSegmentFile(
	ctx context.Context,
	writer io.Writer,
	rangeReader modelpackports.PublishObjectRangeReader,
	file modelpackports.PublishObjectFile,
	currentOffset int64,
	startOffset int64,
	remaining *int64,
	unbounded bool,
) (int64, error) {
	segmentStart, segmentLength, shouldWrite := intersectArchiveSegment(currentOffset, file.SizeBytes, startOffset, *remaining, unbounded)
	if !shouldWrite {
		return file.SizeBytes, nil
	}
	object, err := rangeReader.OpenReadRange(ctx, file.SourcePath, segmentStart, segmentLength)
	if err != nil {
		return 0, err
	}
	defer object.Body.Close()
	if _, err := io.CopyN(writer, object.Body, segmentLength); err != nil {
		return 0, err
	}
	if !unbounded {
		*remaining -= segmentLength
	}
	return file.SizeBytes, nil
}

func writeSegmentReader(
	writer io.Writer,
	reader io.Reader,
	segmentSize int64,
	currentOffset int64,
	startOffset int64,
	remaining *int64,
	unbounded bool,
) (int64, error) {
	segmentStart, segmentLength, shouldWrite := intersectArchiveSegment(currentOffset, segmentSize, startOffset, *remaining, unbounded)
	if !shouldWrite {
		return segmentSize, nil
	}
	if segmentStart > 0 {
		if _, err := io.CopyN(io.Discard, reader, segmentStart); err != nil {
			return 0, err
		}
	}
	if _, err := io.CopyN(writer, reader, segmentLength); err != nil {
		return 0, err
	}
	if !unbounded {
		*remaining -= segmentLength
	}
	return segmentSize, nil
}

func intersectArchiveSegment(
	currentOffset int64,
	segmentSize int64,
	startOffset int64,
	remaining int64,
	unbounded bool,
) (int64, int64, bool) {
	segmentEnd := currentOffset + segmentSize
	if startOffset >= segmentEnd {
		return 0, 0, false
	}
	startWithin := int64(0)
	if startOffset > currentOffset {
		startWithin = startOffset - currentOffset
	}
	available := segmentSize - startWithin
	if available <= 0 {
		return 0, 0, false
	}
	if unbounded || remaining > available {
		return startWithin, available, true
	}
	if remaining <= 0 {
		return 0, 0, false
	}
	return startWithin, remaining, true
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
