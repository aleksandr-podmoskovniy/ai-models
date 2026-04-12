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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/modelformat"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

var (
	huggingFaceBaseURL       = "https://huggingface.co"
	fetchHuggingFaceInfoFunc = FetchHuggingFaceInfo
)

type HuggingFaceInfo struct {
	ID          string
	SHA         string
	PipelineTag string
	License     string
	Files       []string
}

type huggingFaceAPIResponse struct {
	ID          string                   `json:"id"`
	SHA         string                   `json:"sha"`
	PipelineTag string                   `json:"pipeline_tag"`
	CardData    huggingFaceCardData      `json:"cardData"`
	Siblings    []huggingFaceSiblingInfo `json:"siblings"`
}

type huggingFaceSiblingInfo struct {
	Path string `json:"rfilename"`
}

type huggingFaceCardData struct {
	License string `json:"license"`
}

func fetchHuggingFaceModel(ctx context.Context, options RemoteOptions) (RemoteResult, error) {
	repoID, revision, err := modelsv1alpha1.ParseHuggingFaceURL(options.URL)
	if err != nil {
		return RemoteResult{}, err
	}

	info, err := fetchHuggingFaceInfoFunc(ctx, repoID, revision, options.HFToken)
	if err != nil {
		return RemoteResult{}, err
	}

	inputFormat, err := resolveRemoteFormat(info.Files, options.RequestedFormat)
	if err != nil {
		return RemoteResult{}, err
	}
	selectedFiles, err := modelformat.SelectRemoteFiles(inputFormat, info.Files)
	if err != nil {
		return RemoteResult{}, err
	}

	resolvedRevision := ResolveHuggingFaceRevision(info, revision)
	snapshotDir := filepath.Join(options.Workspace, ".hf-snapshot")
	if err := newHuggingFaceSnapshotDownloader().Download(ctx, huggingFaceSnapshotDownloadInput{
		RepoID:      repoID,
		Revision:    resolvedRevision,
		Token:       options.HFToken,
		Files:       selectedFiles,
		SnapshotDir: snapshotDir,
	}); err != nil {
		return RemoteResult{}, err
	}

	modelDir := filepath.Join(options.Workspace, "checkpoint")
	var stagedObjects []cleanuphandle.UploadStagingHandle
	if rawStageEnabled(options.RawStage) {
		stagedObjects, err = stageHuggingFaceSnapshotFiles(ctx, snapshotDir, selectedFiles, *options.RawStage)
		if err != nil {
			return RemoteResult{}, err
		}
	}
	if err := materializeHuggingFaceSnapshot(snapshotDir, modelDir, selectedFiles); err != nil {
		return RemoteResult{}, err
	}
	if err := modelformat.ValidateDir(modelDir, inputFormat); err != nil {
		return RemoteResult{}, err
	}

	return RemoteResult{
		SourceType:  modelsv1alpha1.ModelSourceTypeHuggingFace,
		ModelDir:    modelDir,
		InputFormat: inputFormat,
		Provenance: RemoteProvenance{
			ExternalReference: firstNonEmpty(info.ID, repoID),
			ResolvedRevision:  resolvedRevision,
		},
		ProfileHints: RemoteProfileHints{
			TaskHint:     info.PipelineTag,
			License:      info.License,
			SourceRepoID: info.ID,
		},
		StagedObjects: stagedObjects,
	}, nil
}

func stageHuggingFaceSnapshotFiles(
	ctx context.Context,
	snapshotDir string,
	files []string,
	rawStage RawStageOptions,
) ([]cleanuphandle.UploadStagingHandle, error) {
	if !rawStageEnabled(&rawStage) {
		return nil, nil
	}

	stagedObjects := make([]cleanuphandle.UploadStagingHandle, 0, len(files))
	for _, relativePath := range files {
		cleanPath, err := cleanRemoteRelativePath(relativePath)
		if err != nil {
			return nil, err
		}

		sourcePath := filepath.Join(snapshotDir, cleanPath)
		sourceInfo, err := os.Stat(sourcePath)
		if err != nil {
			return nil, err
		}

		stream, err := os.Open(sourcePath)
		if err != nil {
			return nil, err
		}

		handle, stageErr := stageRawObject(
			ctx,
			rawStage,
			cleanPath,
			filepath.Base(cleanPath),
			sourceInfo.Size(),
			"",
			stream,
		)
		closeErr := stream.Close()
		if stageErr != nil {
			return nil, stageErr
		}
		if closeErr != nil {
			return nil, closeErr
		}

		stagedObjects = append(stagedObjects, handle)
	}

	return stagedObjects, nil
}

func materializeHuggingFaceSnapshot(snapshotDir, destination string, files []string) error {
	for _, relativePath := range files {
		cleanPath, err := cleanRemoteRelativePath(relativePath)
		if err != nil {
			return err
		}
		if err := linkOrCopyFile(filepath.Join(snapshotDir, cleanPath), filepath.Join(destination, cleanPath)); err != nil {
			return err
		}
	}
	return nil
}

func FetchHuggingFaceInfo(ctx context.Context, repoID, revision, token string) (HuggingFaceInfo, error) {
	endpoint, err := huggingFaceInfoURL(repoID, revision)
	if err != nil {
		return HuggingFaceInfo{}, err
	}

	response, err := doGET(ctx, nil, endpoint, bearerAuthHeaders(token))
	if err != nil {
		return HuggingFaceInfo{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return HuggingFaceInfo{}, unexpectedStatusError(response, "huggingface model info request")
	}

	var payload huggingFaceAPIResponse
	if err := decodeJSONResponse(response, &payload); err != nil {
		return HuggingFaceInfo{}, err
	}

	files := make([]string, 0, len(payload.Siblings))
	for _, item := range payload.Siblings {
		if path := strings.TrimSpace(item.Path); path != "" {
			files = append(files, path)
		}
	}

	return HuggingFaceInfo{
		ID:          firstNonEmpty(payload.ID, repoID),
		SHA:         strings.TrimSpace(payload.SHA),
		PipelineTag: strings.TrimSpace(payload.PipelineTag),
		License:     strings.TrimSpace(payload.CardData.License),
		Files:       files,
	}, nil
}

func huggingFaceInfoURL(repoID, revision string) (string, error) {
	repositoryPath, err := huggingFaceRepoPath(repoID)
	if err != nil {
		return "", err
	}
	endpoint := huggingFaceBaseURL + "/api/models/" + repositoryPath
	if strings.TrimSpace(revision) == "" {
		return endpoint, nil
	}
	return endpoint + "?revision=" + url.QueryEscape(strings.TrimSpace(revision)), nil
}

func huggingFaceRepoPath(repoID string) (string, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(repoID), "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("huggingface repo ID must be owner/name, got %q", repoID)
	}
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(part)
		if clean == "" {
			return "", fmt.Errorf("huggingface repo ID contains an empty path segment: %q", repoID)
		}
		escaped = append(escaped, url.PathEscape(clean))
	}
	return strings.Join(escaped, "/"), nil
}

func cleanRemoteRelativePath(path string) (string, error) {
	return archiveRelativePath(strings.ReplaceAll(path, "\\", "/"))
}

func ResolveHuggingFaceRevision(info HuggingFaceInfo, requested string) string {
	if strings.TrimSpace(info.SHA) != "" {
		return strings.TrimSpace(info.SHA)
	}
	if strings.TrimSpace(requested) != "" {
		return strings.TrimSpace(requested)
	}
	return "main"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
