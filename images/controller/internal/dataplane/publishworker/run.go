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
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	ggufprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/gguf"
	safetensorsprofile "github.com/deckhouse/ai-models/controller/internal/adapters/modelprofile/safetensors"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Options struct {
	SourceType         modelsv1alpha1.ModelSourceType
	ArtifactURI        string
	HFModelID          string
	Revision           string
	HTTPURL            string
	HTTPCABundle       []byte
	HTTPAuthDir        string
	UploadPath         string
	UploadStage        *cleanuphandle.UploadStagingHandle
	RawStageBucket     string
	RawStageKeyPrefix  string
	InputFormat        modelsv1alpha1.ModelInputFormat
	Task               string
	RuntimeEngines     []string
	SnapshotDir        string
	HFToken            string
	UploadStaging      uploadstagingports.Client
	ModelPackPublisher modelpackports.Publisher
	RegistryAuth       modelpackports.RegistryAuth
}

func Run(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.ArtifactURI) == "" {
		return publicationartifact.Result{}, errors.New("artifact URI must not be empty")
	}
	if options.ModelPackPublisher == nil {
		return publicationartifact.Result{}, errors.New("ModelPack publisher must not be nil")
	}

	result, err := run(ctx, options)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	return result, nil
}

func publishFromHuggingFace(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.HFModelID) == "" {
		return publicationartifact.Result{}, errors.New("hf-model-id is required")
	}
	remote, cleanupDir, err := fetchRemote(ctx, options, "ai-model-hf-publish-")
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupDir()

	task := firstNonEmpty(options.Task, remote.PipelineTag)
	if task == "" {
		return publicationartifact.Result{}, errors.New("task is required either explicitly or from Hugging Face metadata")
	}

	resolvedProfile, publishResult, err := resolveAndPublish(ctx, options, remote.ModelDir, remote.InputFormat, sourceProfileInput{
		Task:           task,
		Framework:      remote.Framework,
		License:        remote.License,
		SourceRepoID:   remote.SourceRepoID,
		RuntimeEngines: options.RuntimeEngines,
	}, fmt.Sprintf("Published from Hugging Face source %s", options.HFModelID))
	if err != nil {
		return publicationartifact.Result{}, err
	}
	if err := cleanupRemoteStagedObjects(ctx, options, remote.StagedObjects); err != nil {
		return publicationartifact.Result{}, err
	}
	rawSource := remoteRawProvenance(options, remote.StagedObjects)

	return buildBackendResult(
		publicationdata.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHuggingFace,
			ExternalReference: remote.ExternalReference,
			ResolvedRevision:  remote.ResolvedRevision,
			RawURI:            rawSource.RawURI,
			RawObjectCount:    rawSource.RawObjectCount,
			RawSizeBytes:      rawSource.RawSizeBytes,
		},
		resolvedProfile,
		publishResult,
	), nil
}

func publishFromHTTP(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.HTTPURL) == "" {
		return publicationartifact.Result{}, errors.New("http-url is required")
	}
	remote, cleanupDir, err := fetchRemote(ctx, options, "ai-model-http-publish-")
	if err != nil {
		return publicationartifact.Result{}, err
	}
	defer cleanupDir()

	if strings.TrimSpace(options.Task) == "" {
		return publicationartifact.Result{}, errors.New("task is required for HTTP source")
	}

	resolvedProfile, publishResult, err := resolveAndPublish(ctx, options, remote.ModelDir, remote.InputFormat, sourceProfileInput{
		Task:           options.Task,
		Framework:      remote.Framework,
		RuntimeEngines: options.RuntimeEngines,
	}, fmt.Sprintf("Published from HTTP source %s", options.HTTPURL))
	if err != nil {
		return publicationartifact.Result{}, err
	}
	if err := cleanupRemoteStagedObjects(ctx, options, remote.StagedObjects); err != nil {
		return publicationartifact.Result{}, err
	}
	rawSource := remoteRawProvenance(options, remote.StagedObjects)

	return buildBackendResult(
		publicationdata.SourceProvenance{
			Type:              modelsv1alpha1.ModelSourceTypeHTTP,
			ExternalReference: remote.ExternalReference,
			ResolvedRevision:  remote.ResolvedRevision,
			RawURI:            rawSource.RawURI,
			RawObjectCount:    rawSource.RawObjectCount,
			RawSizeBytes:      rawSource.RawSizeBytes,
		},
		resolvedProfile,
		publishResult,
	), nil
}

func fetchRemote(ctx context.Context, options Options, prefix string) (sourcefetch.RemoteResult, func(), error) {
	workspace, cleanupDir, err := ensureWorkspace(options.SnapshotDir, prefix)
	if err != nil {
		return sourcefetch.RemoteResult{}, nil, err
	}

	remoteURL := options.HTTPURL
	if options.SourceType == modelsv1alpha1.ModelSourceTypeHuggingFace {
		remoteURL = huggingFaceSourceURL(options.HFModelID, options.Revision)
	}

	remote, err := sourcefetch.FetchRemoteModel(ctx, sourcefetch.RemoteOptions{
		URL:             remoteURL,
		Workspace:       workspace,
		RequestedFormat: options.InputFormat,
		HFToken:         options.HFToken,
		HTTPCABundle:    options.HTTPCABundle,
		HTTPAuthDir:     options.HTTPAuthDir,
		RawStage:        remoteRawStage(options),
	})
	if err != nil {
		cleanupDir()
		return sourcefetch.RemoteResult{}, nil, err
	}

	return remote, cleanupDir, nil
}

func huggingFaceSourceURL(repoID, revision string) string {
	base := "https://huggingface.co/" + strings.Trim(strings.TrimSpace(repoID), "/")
	if strings.TrimSpace(revision) == "" {
		return base
	}
	return base + "?revision=" + strings.TrimSpace(revision)
}

func resolveAndPublish(
	ctx context.Context,
	options Options,
	checkpointDir string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
	description string,
) (publicationdata.ResolvedProfile, modelpackports.PublishResult, error) {
	resolvedProfile, err := resolveProfile(checkpointDir, inputFormat, input)
	if err != nil {
		return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
	}

	publishResult, err := options.ModelPackPublisher.Publish(ctx, modelpackports.PublishInput{
		ModelDir:    checkpointDir,
		ArtifactURI: options.ArtifactURI,
		Description: description,
	}, options.RegistryAuth)
	if err != nil {
		return publicationdata.ResolvedProfile{}, modelpackports.PublishResult{}, err
	}

	return resolvedProfile, publishResult, nil
}

type sourceProfileInput struct {
	Task           string
	Framework      string
	License        string
	SourceRepoID   string
	RuntimeEngines []string
}

func resolveProfile(
	checkpointDir string,
	inputFormat modelsv1alpha1.ModelInputFormat,
	input sourceProfileInput,
) (publicationdata.ResolvedProfile, error) {
	switch inputFormat {
	case modelsv1alpha1.ModelInputFormatSafetensors:
		return safetensorsprofile.Resolve(safetensorsprofile.Input{
			CheckpointDir:  checkpointDir,
			Task:           input.Task,
			Framework:      input.Framework,
			License:        input.License,
			SourceRepoID:   input.SourceRepoID,
			RuntimeEngines: input.RuntimeEngines,
		})
	case modelsv1alpha1.ModelInputFormatGGUF:
		return ggufprofile.Resolve(ggufprofile.Input{
			ModelDir:       checkpointDir,
			Task:           input.Task,
			SourceRepoID:   input.SourceRepoID,
			RuntimeEngines: input.RuntimeEngines,
		})
	default:
		return publicationdata.ResolvedProfile{}, fmt.Errorf("unsupported model input format %q", inputFormat)
	}
}
