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
	"context"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	"github.com/deckhouse/ai-models/controller/internal/application/sourceadmission"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

func (s *Service) preflight(
	ctx context.Context,
	request publicationports.Request,
	_ publicationapp.SourceWorkerPlan,
) error {
	return sourceadmission.Preflight(ctx, sourceadmission.PreflightInput{
		Owner: ingestadmission.OwnerBinding{
			Kind:      request.Owner.Kind,
			Name:      request.Owner.Name,
			Namespace: request.Owner.Namespace,
			UID:       string(request.Owner.UID),
		},
		Identity: request.Identity,
		Spec:     request.Spec,
	})
}
