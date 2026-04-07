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

package publishplan

import (
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type UploadSessionIssueRequest struct {
	OwnerUID       string
	OwnerKind      string
	OwnerName      string
	Identity       publicationdata.Identity
	Source         modelsv1alpha1.ModelSourceSpec
	InputFormat    modelsv1alpha1.ModelInputFormat
	Task           string
	RuntimeEngines []string
}

type UploadSessionPlan struct {
	InputFormat       modelsv1alpha1.ModelInputFormat
	ExpectedSizeBytes *int64
	Task              string
	RuntimeEngines    []string
}

func IssueUploadSession(request UploadSessionIssueRequest) (UploadSessionPlan, error) {
	if strings.TrimSpace(request.OwnerUID) == "" {
		return UploadSessionPlan{}, errors.New("upload session owner UID must not be empty")
	}
	if strings.TrimSpace(request.OwnerKind) == "" {
		return UploadSessionPlan{}, errors.New("upload session owner kind must not be empty")
	}
	if strings.TrimSpace(request.OwnerName) == "" {
		return UploadSessionPlan{}, errors.New("upload session owner name must not be empty")
	}
	if err := request.Identity.Validate(); err != nil {
		return UploadSessionPlan{}, err
	}
	mode, err := StartPublication(StartPublicationInput{
		Source: request.Source,
		RuntimeHints: &modelsv1alpha1.ModelRuntimeHints{
			Task: request.Task,
		},
	})
	if err != nil {
		return UploadSessionPlan{}, err
	}
	if mode != ExecutionModeUpload {
		return UploadSessionPlan{}, fmt.Errorf("upload session only supports source type %q", modelsv1alpha1.ModelSourceTypeUpload)
	}

	return UploadSessionPlan{
		InputFormat:       request.InputFormat,
		ExpectedSizeBytes: request.Source.Upload.ExpectedSizeBytes,
		Task:              strings.TrimSpace(request.Task),
		RuntimeEngines:    append([]string(nil), request.RuntimeEngines...),
	}, nil
}
