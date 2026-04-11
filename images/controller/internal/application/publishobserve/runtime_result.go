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

package publishobserve

import (
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publishstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func decodeRuntimeResult(
	request publicationports.Request,
	rawResult string,
) (*publicationdomain.PublicationSuccess, error) {
	backendResult, err := publicationartifact.DecodeResult(rawResult)
	if err != nil {
		return nil, err
	}

	return &publicationdomain.PublicationSuccess{
		Snapshot: publication.Snapshot{
			Identity: request.Identity,
			Source:   backendResult.Source,
			Artifact: backendResult.Artifact,
			Resolved: backendResult.Resolved,
			Result: publication.Result{
				State: "Published",
				Ready: true,
			},
		},
		CleanupHandle: backendResult.CleanupHandle,
	}, nil
}
