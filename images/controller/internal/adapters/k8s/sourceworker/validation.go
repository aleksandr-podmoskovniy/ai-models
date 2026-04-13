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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

type Options struct {
	workloadpod.RuntimeOptions
	LogFormat            string
	MaxConcurrentWorkers int
}

func normalizeOptions(options Options) Options {
	options.RuntimeOptions = workloadpod.NormalizeRuntimeOptions(options.RuntimeOptions)
	options.LogFormat = strings.TrimSpace(options.LogFormat)
	if options.LogFormat == "" {
		options.LogFormat = "json"
	}
	if options.MaxConcurrentWorkers <= 0 {
		options.MaxConcurrentWorkers = 1
	}
	return options
}

func validateOptions(plan publicationapp.SourceWorkerPlan, options Options) error {
	if err := validateServiceOptions(options); err != nil {
		return err
	}
	if plan.Upload != nil {
		return storageprojection.ValidateOptions("source worker", options.ObjectStorage)
	}
	return nil
}

func validateServiceOptions(options Options) error {
	if err := workloadpod.ValidateRuntimeOptions("source worker", options.RuntimeOptions); err != nil {
		return err
	}
	if options.MaxConcurrentWorkers <= 0 {
		return errors.New("source worker max concurrent workers must be greater than zero")
	}
	return nil
}

func sourcePlan(request publicationports.Request) (publicationapp.SourceWorkerPlan, error) {
	if err := validateOwner(request.Owner); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	if err := request.Identity.Validate(); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	return publicationapp.PlanSourceWorker(request.Spec, request.Owner.Namespace, request.UploadStage)
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
