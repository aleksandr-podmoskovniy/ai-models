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

package sourceworker

import (
	"strings"

	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"k8s.io/apimachinery/pkg/types"
)

func buildArgs(
	request publicationports.Request,
	plan publicationapp.SourceWorkerPlan,
	artifactURI string,
	options Options,
) []string {
	args := []string{
		"--artifact-uri", artifactURI,
		"--source-type", string(plan.SourceType),
		"--oci-direct-upload-endpoint", strings.TrimSpace(options.OCIDirectUploadEndpoint),
		"--source-acquisition-mode", string(options.SourceAcquisition),
	}
	return append(args, sourceArgs(plan, request.Owner.UID, options.ObjectStorage.Bucket, options.SourceAcquisition)...)
}

func sourceArgs(plan publicationapp.SourceWorkerPlan, ownerUID types.UID, rawBucket string, modes ...publicationports.SourceAcquisitionMode) []string {
	if plan.HuggingFace != nil {
		mode := publicationports.SourceAcquisitionModeDirect
		if len(modes) > 0 {
			mode = publicationports.NormalizeSourceAcquisitionMode(modes[0])
		}
		return append(huggingFaceArgs(plan.HuggingFace), remoteRawStageArgs(ownerUID, rawBucket, mode)...)
	}
	if plan.Upload != nil {
		return uploadArgs(plan.Upload)
	}
	return nil
}

func huggingFaceArgs(source *publicationapp.HuggingFaceSourcePlan) []string {
	args := []string{"--hf-model-id", source.RepoID}
	if strings.TrimSpace(source.Revision) != "" {
		args = append(args, "--revision", source.Revision)
	}
	return args
}

func uploadArgs(source *publicationapp.UploadSourcePlan) []string {
	args := []string{
		"--upload-stage-bucket", source.Stage.Bucket,
		"--upload-stage-key", source.Stage.Key,
	}
	if strings.TrimSpace(source.Stage.FileName) != "" {
		args = append(args, "--upload-stage-file-name", source.Stage.FileName)
	}
	return args
}

func remoteRawStageArgs(ownerUID types.UID, rawBucket string, mode publicationports.SourceAcquisitionMode) []string {
	if publicationports.NormalizeSourceAcquisitionMode(mode) != publicationports.SourceAcquisitionModeMirror {
		return nil
	}
	if strings.TrimSpace(rawBucket) == "" {
		return nil
	}

	keyPrefix, err := resourcenames.UploadStagingObjectPrefix(ownerUID)
	if err != nil {
		return nil
	}

	return []string{
		"--raw-stage-bucket", rawBucket,
		"--raw-stage-key-prefix", keyPrefix + "/source-url",
	}
}
