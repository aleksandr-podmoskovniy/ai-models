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
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func sourceMirrorSupportsStreamingPublish(options Options) bool {
	return remoteSourceMirror(options) != nil
}

func buildHuggingFacePublishLayers(
	ctx context.Context,
	options Options,
	remote sourcefetch.RemoteResult,
) ([]modelpackports.PublishLayer, error) {
	if remote.ObjectSource != nil {
		return []modelpackports.PublishLayer{
			{
				SourcePath:  huggingFaceArtifactURI(remote),
				TargetPath:  modelpackports.MaterializedModelPathName,
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatTar,
				Compression: modelpackports.LayerCompressionNone,
				ObjectSource: &modelpackports.PublishObjectSource{
					Reader: remoteObjectReader{reader: remote.ObjectSource.Reader},
					Files:  mapRemoteObjectFiles(remote.ObjectSource.Files),
				},
			},
		}, nil
	}
	if strings.TrimSpace(remote.ModelDir) != "" || remote.SourceMirror == nil {
		return nil, nil
	}

	files, err := sourceMirrorPublishFiles(ctx, options, remote.SourceMirror, remote.SelectedFiles)
	if err != nil {
		return nil, err
	}
	return []modelpackports.PublishLayer{
		{
			SourcePath:  sourceMirrorArtifactURI(options, remote.SourceMirror),
			TargetPath:  modelpackports.MaterializedModelPathName,
			Base:        modelpackports.LayerBaseModel,
			Format:      modelpackports.LayerFormatTar,
			Compression: modelpackports.LayerCompressionNone,
			ObjectSource: &modelpackports.PublishObjectSource{
				Reader: uploadStagingObjectReader{
					bucket: strings.TrimSpace(options.RawStageBucket),
					reader: options.UploadStaging,
				},
				Files: files,
			},
		},
	}, nil
}

func mapRemoteObjectFiles(files []sourcefetch.RemoteObjectFile) []modelpackports.PublishObjectFile {
	mapped := make([]modelpackports.PublishObjectFile, 0, len(files))
	for _, file := range files {
		mapped = append(mapped, modelpackports.PublishObjectFile{
			SourcePath: strings.TrimSpace(file.SourcePath),
			TargetPath: strings.TrimSpace(file.TargetPath),
			SizeBytes:  file.SizeBytes,
			ETag:       strings.TrimSpace(file.ETag),
		})
	}
	return mapped
}

func sourceMirrorPublishFiles(
	ctx context.Context,
	options Options,
	snapshot *sourcefetch.SourceMirrorSnapshot,
	selectedFiles []string,
) ([]modelpackports.PublishObjectFile, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("source mirror snapshot must not be nil")
	}
	files := make([]modelpackports.PublishObjectFile, 0, len(selectedFiles))
	for _, filePath := range selectedFiles {
		cleanPath, err := cleanMirrorRelativePath(filePath)
		if err != nil {
			return nil, err
		}
		key := sourcemirrorports.SnapshotFileObjectKey(snapshot.CleanupPrefix, cleanPath)
		stat, err := options.UploadStaging.Stat(ctx, uploadstagingports.StatInput{
			Bucket: strings.TrimSpace(options.RawStageBucket),
			Key:    key,
		})
		if err != nil {
			return nil, err
		}
		files = append(files, modelpackports.PublishObjectFile{
			SourcePath: key,
			TargetPath: cleanPath,
			SizeBytes:  stat.SizeBytes,
			ETag:       stat.ETag,
		})
	}
	return files, nil
}

func cleanMirrorRelativePath(raw string) (string, error) {
	clean := path.Clean(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")))
	switch {
	case clean == "", clean == ".":
		return "", fmt.Errorf("source mirror relative path must not be empty")
	case strings.HasPrefix(clean, "/"):
		return "", fmt.Errorf("source mirror relative path must be relative, got %q", raw)
	case strings.HasPrefix(clean, "../") || clean == "..":
		return "", fmt.Errorf("source mirror relative path must not escape snapshot root, got %q", raw)
	default:
		return filepath.ToSlash(clean), nil
	}
}

func sourceMirrorArtifactURI(options Options, snapshot *sourcefetch.SourceMirrorSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return "s3://" + path.Join(strings.TrimSpace(options.RawStageBucket), strings.Trim(strings.TrimSpace(snapshot.CleanupPrefix), "/"))
}

func huggingFaceArtifactURI(remote sourcefetch.RemoteResult) string {
	repoID := strings.Trim(strings.TrimSpace(remote.Provenance.ExternalReference), "/")
	if repoID == "" {
		return ""
	}
	artifactURI := "https://huggingface.co/" + repoID
	if revision := strings.TrimSpace(remote.Provenance.ResolvedRevision); revision != "" {
		artifactURI += "?revision=" + revision
	}
	return artifactURI
}

type uploadStagingObjectReader struct {
	bucket string
	reader uploadstagingports.Reader
}

func (r uploadStagingObjectReader) OpenRead(ctx context.Context, sourcePath string) (modelpackports.OpenReadResult, error) {
	output, err := r.reader.OpenRead(ctx, uploadstagingports.OpenReadInput{
		Bucket: strings.TrimSpace(r.bucket),
		Key:    strings.TrimSpace(sourcePath),
	})
	if err != nil {
		return modelpackports.OpenReadResult{}, err
	}
	return modelpackports.OpenReadResult{
		Body:      output.Body,
		SizeBytes: output.SizeBytes,
		ETag:      output.ETag,
	}, nil
}

func (r uploadStagingObjectReader) OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (modelpackports.OpenReadResult, error) {
	rangeReader, ok := r.reader.(uploadstagingports.RangeReader)
	if !ok {
		return r.OpenRead(ctx, sourcePath)
	}
	output, err := rangeReader.OpenReadRange(ctx, uploadstagingports.OpenReadRangeInput{
		Bucket: strings.TrimSpace(r.bucket),
		Key:    strings.TrimSpace(sourcePath),
		Offset: offset,
		Length: length,
	})
	if err != nil {
		return modelpackports.OpenReadResult{}, err
	}
	return modelpackports.OpenReadResult{
		Body:      output.Body,
		SizeBytes: output.SizeBytes,
		ETag:      output.ETag,
	}, nil
}

type remoteObjectReader struct {
	reader sourcefetch.RemoteObjectReader
}

func (r remoteObjectReader) OpenRead(ctx context.Context, sourcePath string) (modelpackports.OpenReadResult, error) {
	output, err := r.reader.OpenRead(ctx, strings.TrimSpace(sourcePath))
	if err != nil {
		return modelpackports.OpenReadResult{}, err
	}
	return modelpackports.OpenReadResult{
		Body:      output.Body,
		SizeBytes: output.SizeBytes,
		ETag:      output.ETag,
	}, nil
}

func (r remoteObjectReader) OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (modelpackports.OpenReadResult, error) {
	rangeReader, ok := r.reader.(sourcefetch.RemoteObjectRangeReader)
	if !ok {
		return r.OpenRead(ctx, sourcePath)
	}
	output, err := rangeReader.OpenReadRange(ctx, strings.TrimSpace(sourcePath), offset, length)
	if err != nil {
		return modelpackports.OpenReadResult{}, err
	}
	return modelpackports.OpenReadResult{
		Body:      output.Body,
		SizeBytes: output.SizeBytes,
		ETag:      output.ETag,
	}, nil
}
