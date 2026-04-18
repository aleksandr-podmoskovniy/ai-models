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
	"log/slog"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type sourceProfileInput struct {
	Task       string
	TaskHint   string
	Provenance sourceProfileProvenance
}

type sourceProfileProvenance struct {
	License      string
	SourceRepoID string
}

func resolveAndPublish(
	ctx context.Context,
	options Options,
	modelInputPath string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
	preResolved *publicationdata.ResolvedProfile,
) (publicationdata.ResolvedProfile, modelpackports.PublishResult, error) {
	return resolveAndPublishWithLayers(ctx, options, modelInputPath, inputFormat, input, nil, preResolved)
}

func resolveAndPublishWithLayers(
	ctx context.Context,
	options Options,
	modelInputPath string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
	publishLayers []modelpackports.PublishLayer,
	preResolved *publicationdata.ResolvedProfile,
) (publicationdata.ResolvedProfile, modelpackports.PublishResult, error) {
	logger := slog.Default().With(
		slog.String("modelInputPath", strings.TrimSpace(modelInputPath)),
		slog.String("resolvedInputFormat", strings.TrimSpace(string(inputFormat))),
	)

	var (
		resolvedProfile publicationdata.ResolvedProfile
		err             error
	)
	resolveStarted := time.Now()
	if preResolved != nil {
		logger.Info("publication profile resolution reused precomputed summary")
		resolvedProfile = *preResolved
	} else {
		logger.Info("publication profile resolution started")
		resolvedProfile, err = resolveProfile(modelInputPath, inputFormat, input)
		if err != nil {
			return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
		}
	}
	resolvedProfile = attachResolvedProfileProvenance(resolvedProfile, input.Provenance)
	logger.Info(
		"publication profile resolution completed",
		slog.Int64("durationMs", time.Since(resolveStarted).Milliseconds()),
		slog.String("resolvedTask", strings.TrimSpace(resolvedProfile.Task)),
		slog.String("resolvedFamily", strings.TrimSpace(resolvedProfile.Family)),
		slog.Int("compatibleRuntimeCount", len(resolvedProfile.CompatibleRuntimes)),
		slog.Int("supportedEndpointTypeCount", len(resolvedProfile.SupportedEndpointTypes)),
		slog.Int64("parameterCount", resolvedProfile.ParameterCount),
	)

	publishStarted := time.Now()
	logger.Info("modelpack publication started", slog.String("artifactURI", strings.TrimSpace(options.ArtifactURI)))
	publishInput := modelpackports.PublishInput{
		ModelDir:    modelInputPath,
		Layers:      publishLayers,
		ArtifactURI: options.ArtifactURI,
	}
	publishResult, err := options.ModelPackPublisher.Publish(ctx, publishInput, options.RegistryAuth)
	if err != nil {
		return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
	}
	logger.Info(
		"modelpack publication completed",
		slog.Int64("durationMs", time.Since(publishStarted).Milliseconds()),
		slog.String("artifactDigest", strings.TrimSpace(publishResult.Digest)),
		slog.String("artifactMediaType", strings.TrimSpace(publishResult.MediaType)),
		slog.Int64("artifactSizeBytes", publishResult.SizeBytes),
	)

	return resolvedProfile, publishResult, nil
}

func resolveProfile(
	checkpointDir string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
) (publicationdata.ResolvedProfile, error) {
	switch inputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsprofile.Resolve(safetensorsprofile.Input{
			CheckpointDir: checkpointDir,
			Task:          input.Task,
			TaskHint:      input.TaskHint,
		})
	case modelsv1alpha1.ModelInputFormatGGUF:
		return ggufprofile.Resolve(ggufprofile.Input{
			ModelDir: checkpointDir,
			Task:     input.Task,
		})
	default:
		return publicationdata.ResolvedProfile{}, fmt.Errorf("unsupported model input format %q", inputFormat)
	}
}

func attachResolvedProfileProvenance(
	resolved publicationdata.ResolvedProfile,
	provenance sourceProfileProvenance,
) publicationdata.ResolvedProfile {
	resolved.License = strings.TrimSpace(provenance.License)
	resolved.SourceRepoID = strings.TrimSpace(provenance.SourceRepoID)
	return resolved
}
