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
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

type Options = workloadpod.RuntimeOptions

func validateOptions(options Options) error {
	return workloadpod.ValidateRuntimeOptions("source worker", options)
}

func sourcePlan(request publicationports.OperationContext) (publicationapp.SourceWorkerPlan, error) {
	if err := validateOwner(request.Request.Owner); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	if err := request.Request.Identity.Validate(); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	return publicationapp.PlanSourceWorker(request.Request.Spec, request.Request.Owner.Namespace)
}

func validateOwner(owner publicationports.Owner) error {
	switch {
	case strings.TrimSpace(string(owner.UID)) == "":
		return errors.New("source worker owner UID must not be empty")
	case strings.TrimSpace(owner.Kind) == "":
		return errors.New("source worker owner kind must not be empty")
	case strings.TrimSpace(owner.Name) == "":
		return errors.New("source worker owner name must not be empty")
	default:
		return nil
	}
}
