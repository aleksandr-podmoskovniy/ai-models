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

package sourceadmission

import (
	"context"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type PreflightInput struct {
	Owner    ingestadmission.OwnerBinding
	Identity publicationdata.Identity
	Spec     modelsv1alpha1.ModelSpec
}

func Preflight(ctx context.Context, input PreflightInput) error {
	if err := ingestadmission.ValidateOwnerBinding(input.Owner, input.Identity); err != nil {
		return err
	}
	if err := ingestadmission.ValidateDeclaredInputFormat(input.Spec.InputFormat); err != nil {
		return err
	}

	sourceType, err := input.Spec.Source.DetectType()
	if err != nil {
		return err
	}
	_ = sourceType
	return nil
}
