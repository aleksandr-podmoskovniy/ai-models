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
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

func fetchOllamaModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	ref, err := parseOllamaReference(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	logger := slog.Default().With(
		slog.String("sourceType", string(modelsv1alpha1.ModelSourceTypeOllama)),
		slog.String("sourceRepoID", ref.Subject),
		slog.String("requestedRevision", ref.Tag),
	)
	client := ollamaRegistryClient{baseURL: ollamaRegistryBaseURL, httpClient: http.DefaultClient}

	started := time.Now()
	logger.Info("ollama manifest request started")
	manifest, err := client.fetchManifest(ctx, ref)
	if err != nil {
		return RemoteResult{}, err
	}
	modelLayer, err := selectOllamaModelLayer(manifest)
	if err != nil {
		return RemoteResult{}, err
	}
	logger.Info(
		"ollama manifest request completed",
		slog.Int64("durationMs", time.Since(started).Milliseconds()),
		slog.String("modelLayerDigest", modelLayer.Digest),
		slog.Int64("modelLayerSizeBytes", modelLayer.Size),
	)

	configPayload, err := client.fetchBlobBytes(ctx, ref, manifest.Config, ollamaConfigMaxBytes)
	if err != nil {
		return RemoteResult{}, fmt.Errorf("ollama config fetch failed: %w", err)
	}
	config, err := decodeOllamaConfig(configPayload)
	if err != nil {
		return RemoteResult{}, err
	}
	params, err := fetchOllamaParams(ctx, client, ref, manifest)
	if err != nil {
		return RemoteResult{}, err
	}
	license, err := fetchOllamaLicense(ctx, client, ref, manifest)
	if err != nil {
		return RemoteResult{}, err
	}
	if err := client.probeGGUF(ctx, ref, modelLayer); err != nil {
		return RemoteResult{}, err
	}

	plannedFiles := []RemoteObjectFile{{
		SourcePath: client.blobURL(ref, modelLayer.Digest),
		TargetPath: ollamaModelFileName(ref, config),
		SizeBytes:  modelLayer.Size,
	}}
	if err := reserveOllamaStorage(ctx, options, ref, plannedFiles); err != nil {
		return RemoteResult{}, err
	}

	sourceMirrorSnapshot, err := prepareOllamaSourceMirror(ctx, options, ref, plannedFiles)
	if err != nil {
		return RemoteResult{}, err
	}
	modelDir, objectSource, err := prepareOllamaPublishSource(ctx, options, plannedFiles, sourceMirrorSnapshot)
	if err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:     modelsv1alpha1.ModelSourceTypeOllama,
		ModelDir:       modelDir,
		InputFormat:    modelsv1alpha1.ModelInputFormatGGUF,
		SelectedFiles:  []string{plannedFiles[0].TargetPath},
		ObjectSource:   objectSource,
		ProfileSummary: ollamaProfileSummary(config, params, plannedFiles[0]),
		Provenance: RemoteProvenance{
			ExternalReference: ref.ExternalReference,
			ResolvedRevision:  ref.Tag + "@" + modelLayer.Digest,
		},
		Metadata: RemoteMetadata{
			License:      license,
			SourceRepoID: ref.Subject,
		},
		SourceMirror: sourceMirrorSnapshot,
	}, nil
}

func fetchOllamaParams(
	ctx context.Context,
	client ollamaRegistryClient,
	ref ollamaReference,
	manifest ollamaManifest,
) (ollamaParams, error) {
	layer, found, err := selectOllamaLayer(manifest, ollamaParamsLayerMediaType)
	if err != nil || !found {
		return ollamaParams{}, err
	}
	payload, err := client.fetchBlobBytes(ctx, ref, layer, ollamaParamsMaxBytes)
	if err != nil {
		return ollamaParams{}, fmt.Errorf("ollama params fetch failed: %w", err)
	}
	return decodeOllamaParams(payload)
}

func fetchOllamaLicense(
	ctx context.Context,
	client ollamaRegistryClient,
	ref ollamaReference,
	manifest ollamaManifest,
) (string, error) {
	layer, found, err := selectOllamaLayer(manifest, ollamaLicenseLayerMediaType)
	if err != nil || !found {
		return "", err
	}
	payload, err := client.fetchBlobBytes(ctx, ref, layer, ollamaLicenseMaxBytes)
	if err != nil {
		return "", fmt.Errorf("ollama license fetch failed: %w", err)
	}
	return strings.TrimSpace(string(payload)), nil
}

func ollamaProfileSummary(config ollamaConfig, params ollamaParams, file RemoteObjectFile) *RemoteProfileSummary {
	return &RemoteProfileSummary{
		ModelFileName:       file.TargetPath,
		ModelSizeBytes:      file.SizeBytes,
		Family:              firstNonEmpty(config.ModelFamily, firstString(config.ModelFamilies)),
		ParameterCount:      parseOllamaParameterCount(config.ModelType),
		Quantization:        config.FileType,
		ContextWindowTokens: params.NumCtx,
	}
}

func prepareOllamaPublishSource(
	ctx context.Context,
	options RemoteOptions,
	plannedFiles []RemoteObjectFile,
	sourceMirrorSnapshot *SourceMirrorSnapshot,
) (string, *RemoteObjectSource, error) {
	if sourceMirrorSnapshot != nil {
		if err := transferOllamaMirrorSnapshot(ctx, options, plannedFiles, sourceMirrorSnapshot); err != nil {
			_ = persistOllamaMirrorPhase(ctx, options.SourceMirror, sourceMirrorSnapshot, sourcemirrorports.SnapshotPhaseFailed, plannedFiles, err.Error())
			return "", nil, err
		}
		return "", nil, nil
	}
	if !options.SkipLocalMaterialization {
		return "", nil, fmt.Errorf("ollama remote publication requires direct object-source streaming")
	}
	return "", &RemoteObjectSource{
		Reader: ollamaObjectReader{httpClient: http.DefaultClient},
		Files:  plannedFiles,
	}, nil
}

func reserveOllamaStorage(ctx context.Context, options RemoteOptions, ref ollamaReference, files []RemoteObjectFile) error {
	if options.StorageReservation == nil {
		return nil
	}
	var sizeBytes int64
	for _, file := range files {
		sizeBytes += file.SizeBytes
	}
	mode := "direct"
	if options.SourceMirror != nil {
		mode = "mirror"
	}
	return options.StorageReservation.ReserveRemoteStorage(ctx, RemoteStorageReservationRequest{
		SourceType:        modelsv1alpha1.ModelSourceTypeOllama,
		SourceFetchMode:   mode,
		ExternalReference: ref.ExternalReference,
		ResolvedRevision:  ref.Tag,
		SelectedFileCount: len(files),
		SizeBytes:         sizeBytes,
	})
}

func ollamaModelFileName(ref ollamaReference, config ollamaConfig) string {
	name := sanitizeOllamaFilePart(ref.Name)
	tag := sanitizeOllamaFilePart(ref.Tag)
	quantization := strings.ToLower(sanitizeOllamaFilePart(config.FileType))
	parts := []string{name, tag}
	if quantization != "" {
		parts = append(parts, quantization)
	}
	return path.Clean(strings.Join(parts, "-") + ".gguf")
}

func sanitizeOllamaFilePart(value string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-", " ", "-").Replace(strings.TrimSpace(value))
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
