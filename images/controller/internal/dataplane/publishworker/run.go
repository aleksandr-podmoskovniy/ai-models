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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Options struct {
	SourceType              modelsv1alpha1.ModelSourceType
	ArtifactURI             string
	HFModelID               string
	OCIDirectUploadEndpoint string
	DirectUploadCAFile      string
	DirectUploadInsecure    bool
	SourceFetchMode         publicationports.SourceFetchMode
	Revision                string
	UploadPath              string
	UploadStage             *cleanuphandle.UploadStagingHandle
	RawStageBucket          string
	RawStageKeyPrefix       string
	InputFormat             modelsv1alpha1.ModelInputFormat
	Task                    string
	HFToken                 string
	UploadStaging           uploadStagingClient
	StorageReservation      sourcefetch.RemoteStorageReservation
	ModelPackPublisher      modelpackports.Publisher
	RegistryAuth            modelpackports.RegistryAuth
	DirectUploadState       modelpackports.DirectUploadStateStore
}

func Run(ctx context.Context, options Options) (publicationartifact.Result, error) {
	if strings.TrimSpace(options.ArtifactURI) == "" {
		return publicationartifact.Result{}, errors.New("artifact URI must not be empty")
	}
	if options.ModelPackPublisher == nil {
		return publicationartifact.Result{}, errors.New("ModelPack publisher must not be nil")
	}
	if strings.TrimSpace(options.OCIDirectUploadEndpoint) == "" {
		return publicationartifact.Result{}, errors.New("OCI direct upload endpoint must not be empty")
	}
	options.SourceFetchMode = publicationports.NormalizeSourceFetchMode(options.SourceFetchMode)
	if err := publicationports.ValidateSourceFetchMode(options.SourceFetchMode); err != nil {
		return publicationartifact.Result{}, err
	}
	if options.SourceType == modelsv1alpha1.ModelSourceTypeHuggingFace {
		if options.SourceFetchMode == publicationports.SourceFetchModeMirror {
			switch {
			case strings.TrimSpace(options.RawStageBucket) == "":
				return publicationartifact.Result{}, errors.New("mirror source fetch requires raw stage bucket")
			case strings.TrimSpace(options.RawStageKeyPrefix) == "":
				return publicationartifact.Result{}, errors.New("mirror source fetch requires raw stage key prefix")
			case options.UploadStaging == nil:
				return publicationartifact.Result{}, errors.New("mirror source fetch requires upload staging client")
			}
		}
		if options.SourceFetchMode == publicationports.SourceFetchModeDirect {
			options.RawStageBucket = ""
			options.RawStageKeyPrefix = ""
		}
	}

	result, err := run(ctx, options)
	if err != nil {
		return publicationartifact.Result{}, err
	}
	return result, nil
}
