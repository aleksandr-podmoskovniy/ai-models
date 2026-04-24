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

package archiveio

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func NewClosableTarReader(path string, stream io.Reader) (*tar.Reader, func() error, error) {
	lowerPath := strings.ToLower(strings.TrimSpace(path))
	if strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz") {
		gzipReader, err := gzip.NewReader(stream)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(gzipReader), gzipReader.Close, nil
	}
	if strings.HasSuffix(lowerPath, ".tar.zst") || strings.HasSuffix(lowerPath, ".tar.zstd") || strings.HasSuffix(lowerPath, ".tzst") {
		decoder, err := zstd.NewReader(stream)
		if err != nil {
			return nil, nil, err
		}
		return tar.NewReader(decoder), func() error {
			decoder.Close()
			return nil
		}, nil
	}
	return tar.NewReader(stream), func() error { return nil }, nil
}

func IsTarArchive(path string) bool {
	lowerPath := strings.ToLower(strings.TrimSpace(path))
	return strings.HasSuffix(lowerPath, ".tar") ||
		strings.HasSuffix(lowerPath, ".tar.gz") ||
		strings.HasSuffix(lowerPath, ".tgz") ||
		strings.HasSuffix(lowerPath, ".tar.zst") ||
		strings.HasSuffix(lowerPath, ".tar.zstd") ||
		strings.HasSuffix(lowerPath, ".tzst")
}

func IsZipArchive(path string) bool {
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(path)), ".zip")
}

func IsZipSymlink(file interface{ Mode() os.FileMode }) bool {
	return file.Mode()&os.ModeSymlink != 0
}

func ExtractTarEntry(reader *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		return WriteExtractedFile(target, reader)
	case tar.TypeSymlink:
		return fmt.Errorf("refusing to extract symbolic link tar entry %q", header.Name)
	case tar.TypeLink:
		return fmt.Errorf("refusing to extract hard link tar entry %q", header.Name)
	default:
		return fmt.Errorf("refusing to extract unsupported tar entry %q", header.Name)
	}
}

func WriteExtractedFile(target string, reader io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	stream, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(stream, reader); err != nil {
		stream.Close()
		return err
	}
	return stream.Close()
}

func TargetPath(destination, name string) (string, error) {
	relative, err := RelativePath(name)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return destination, nil
	}
	return filepath.Join(destination, relative), nil
}

func RelativePath(name string) (string, error) {
	rawName := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if rawName == "" {
		return "", errors.New("archive entry name must not be empty")
	}

	parts := strings.Split(rawName, "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("refusing to extract archive entry outside of destination: %q", name)
		default:
			result = append(result, part)
		}
	}
	if len(result) == 0 {
		return ".", nil
	}
	return filepath.Join(result...), nil
}

type RangeOpenFunc func(ctx context.Context, sourcePath string, offset, length int64) (io.ReadCloser, error)

type RangeReaderAt struct {
	ctx        context.Context
	sourcePath string
	sizeBytes  int64
	openRange  RangeOpenFunc
}

func NewRangeReaderAt(ctx context.Context, sourcePath string, sizeBytes int64, openRange RangeOpenFunc) RangeReaderAt {
	return RangeReaderAt{
		ctx:        ctx,
		sourcePath: strings.TrimSpace(sourcePath),
		sizeBytes:  sizeBytes,
		openRange:  openRange,
	}
}

func (r RangeReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.openRange == nil {
		return 0, errors.New("archive range reader must not be nil")
	}
	if offset < 0 || offset >= r.sizeBytes {
		return 0, io.EOF
	}

	length := int64(len(p))
	if remaining := r.sizeBytes - offset; length > remaining {
		length = remaining
	}
	stream, err := r.openRange(r.ctx, r.sourcePath, offset, length)
	if err != nil {
		return 0, err
	}
	defer stream.Close()

	n, err := io.ReadFull(stream, p[:length])
	switch err {
	case nil:
		if int64(n) < int64(len(p)) {
			return n, io.EOF
		}
		return n, nil
	case io.ErrUnexpectedEOF, io.EOF:
		if n == 0 {
			return 0, io.EOF
		}
		return n, io.EOF
	default:
		return n, err
	}
}
