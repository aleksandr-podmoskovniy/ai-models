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
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func failFastUploadProbe(ctx context.Context, options Options, logger *slog.Logger) error {
	fileName, chunk, probed, err := readUploadProbeChunk(ctx, options)
	if err != nil || !probed {
		return err
	}
	if _, err := ingestadmission.ValidateUploadProbeShape(options.InputFormat, ingestadmission.UploadProbeInput{
		FileName: fileName,
		Chunk:    chunk,
	}); err != nil {
		return err
	}
	logger.Debug(
		"upload probe validated before workspace preparation",
		slog.String("fileName", fileName),
		slog.Int("probeBytes", len(chunk)),
	)
	return nil
}

func readUploadProbeChunk(ctx context.Context, options Options) (string, []byte, bool, error) {
	if strings.TrimSpace(options.UploadPath) != "" {
		fileName := filepath.Base(strings.TrimSpace(options.UploadPath))
		chunk, err := readLocalUploadProbeChunk(strings.TrimSpace(options.UploadPath))
		return fileName, chunk, true, err
	}
	if options.UploadStage == nil {
		return "", nil, false, nil
	}
	fileName, err := uploadStageFileName(options)
	if err != nil {
		return "", nil, false, err
	}
	chunk, err := readUploadStageProbeChunk(ctx, options, options.UploadStaging)
	return fileName, chunk, true, err
}

func readLocalUploadProbeChunk(path string) ([]byte, error) {
	stream, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	return readProbeBytes(stream)
}

func readUploadStageProbeChunk(
	ctx context.Context,
	options Options,
	reader uploadstagingports.Reader,
) ([]byte, error) {
	if rangeReader, ok := reader.(uploadstagingports.RangeReader); ok {
		output, err := rangeReader.OpenReadRange(ctx, uploadstagingports.OpenReadRangeInput{
			Bucket: strings.TrimSpace(options.UploadStage.Bucket),
			Key:    strings.TrimSpace(options.UploadStage.Key),
			Offset: 0,
			Length: ingestadmission.MaxUploadProbeBytes,
		})
		if err != nil {
			return nil, err
		}
		defer output.Body.Close()
		return readProbeBytes(output.Body)
	}

	output, err := reader.OpenRead(ctx, uploadstagingports.OpenReadInput{
		Bucket: strings.TrimSpace(options.UploadStage.Bucket),
		Key:    strings.TrimSpace(options.UploadStage.Key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	return readProbeBytes(output.Body)
}

func readProbeBytes(stream io.Reader) ([]byte, error) {
	buffer := make([]byte, ingestadmission.MaxUploadProbeBytes)
	n, err := io.ReadFull(stream, buffer)
	switch err {
	case nil:
		return buffer[:n], nil
	case io.EOF, io.ErrUnexpectedEOF:
		return buffer[:n], nil
	default:
		return nil, err
	}
}
