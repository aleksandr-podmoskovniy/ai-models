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

package uploadsession

import (
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
)

func requestPlan(request publicationports.OperationContext) (publicationapp.UploadSessionPlan, error) {
	task := ""
	if request.Request.Spec.RuntimeHints != nil {
		task = request.Request.Spec.RuntimeHints.Task
	}

	return publicationapp.IssueUploadSession(publicationapp.UploadSessionIssueRequest{
		OwnerUID:           string(request.Request.Owner.UID),
		OwnerKind:          request.Request.Owner.Kind,
		OwnerName:          request.Request.Owner.Name,
		Identity:           request.Request.Identity,
		OperationName:      request.OperationName,
		OperationNamespace: request.OperationNamespace,
		SourceType:         request.Request.Spec.Source.Type,
		Upload:             request.Request.Spec.Source.Upload,
		Task:               task,
	})
}
