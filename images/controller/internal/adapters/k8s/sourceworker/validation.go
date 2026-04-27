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
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Options struct {
	RuntimeOptions
	LogFormat            string
	LogLevel             string
	MaxConcurrentWorkers int
}

type SourceWorkerPlan struct {
	SourceType  modelsv1alpha1.ModelSourceType
	HuggingFace *HuggingFaceSourcePlan
	Upload      *UploadSourcePlan
}

type SourceAuthSecretRef struct {
	Namespace string
	Name      string
}

type HuggingFaceSourcePlan struct {
	RepoID        string
	Revision      string
	AuthSecretRef *SourceAuthSecretRef
}

type UploadSourcePlan struct {
	Stage cleanuphandle.UploadStagingHandle
}

func normalizeOptions(options Options) Options {
	options.RuntimeOptions = NormalizeRuntimeOptions(options.RuntimeOptions)
	options.LogFormat = strings.TrimSpace(options.LogFormat)
	if options.LogFormat == "" {
		options.LogFormat = "json"
	}
	options.LogLevel = strings.TrimSpace(options.LogLevel)
	if options.LogLevel == "" {
		options.LogLevel = "info"
	}
	if options.MaxConcurrentWorkers <= 0 {
		options.MaxConcurrentWorkers = 1
	}
	return options
}

func validateOptions(plan SourceWorkerPlan, options Options) error {
	if err := validateServiceOptions(options); err != nil {
		return err
	}
	if sourceUsesObjectStorage(plan, options) {
		return storageprojection.ValidateOptions("source worker", options.ObjectStorage)
	}
	return nil
}

func validateServiceOptions(options Options) error {
	if err := ValidateRuntimeOptions("source worker", options.RuntimeOptions); err != nil {
		return err
	}
	if options.MaxConcurrentWorkers <= 0 {
		return errors.New("source worker max concurrent workers must be greater than zero")
	}
	return nil
}

func sourcePlan(request publicationports.Request) (SourceWorkerPlan, error) {
	if err := validateOwner(request.Owner); err != nil {
		return SourceWorkerPlan{}, err
	}
	if err := request.Identity.Validate(); err != nil {
		return SourceWorkerPlan{}, err
	}
	return planSourceWorker(request.Spec, request.Owner.Namespace, request.UploadStage)
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

func planSourceWorker(
	spec modelsv1alpha1.ModelSpec,
	ownerNamespace string,
	uploadStage *cleanuphandle.UploadStagingHandle,
) (SourceWorkerPlan, error) {
	sourceType, err := modelsource.DetectType(spec.Source)
	if err != nil {
		return SourceWorkerPlan{}, err
	}
	plan := SourceWorkerPlan{SourceType: sourceType}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		repoID, revision, err := modelsource.ParseHuggingFaceURL(spec.Source.URL)
		if err != nil {
			return SourceWorkerPlan{}, err
		}
		authSecretRef, err := resolveSourceAuthSecretRef(spec.Source.AuthSecretRef, ownerNamespace, "huggingFace")
		if err != nil {
			return SourceWorkerPlan{}, err
		}
		plan.HuggingFace = &HuggingFaceSourcePlan{
			RepoID:        repoID,
			Revision:      revision,
			AuthSecretRef: authSecretRef,
		}
		return plan, nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		if uploadStage == nil {
			return SourceWorkerPlan{}, errors.New("source worker upload source requires a staged upload handle")
		}
		plan.Upload = &UploadSourcePlan{Stage: *uploadStage}
		return plan, nil
	default:
		return SourceWorkerPlan{}, fmt.Errorf("source worker does not support source type %q", sourceType)
	}
}

func resolveSourceAuthSecretRef(ref *modelsv1alpha1.SecretReference, ownerNamespace, sourceKind string) (*SourceAuthSecretRef, error) {
	if ref == nil {
		return nil, nil
	}

	name := strings.TrimSpace(ref.Name)
	if name == "" {
		return nil, fmt.Errorf("source worker %s authSecretRef name must not be empty", sourceKind)
	}

	namespace := strings.TrimSpace(ref.Namespace)
	resolvedOwnerNamespace := strings.TrimSpace(ownerNamespace)
	if resolvedOwnerNamespace != "" {
		switch {
		case namespace == "":
			namespace = resolvedOwnerNamespace
		case namespace != resolvedOwnerNamespace:
			return nil, fmt.Errorf("source worker %s authSecretRef namespace must match owner namespace %q", sourceKind, resolvedOwnerNamespace)
		}
	} else {
		return nil, fmt.Errorf("source worker %s authSecretRef is not supported for cluster-scoped owners", sourceKind)
	}
	if namespace == "" {
		return nil, fmt.Errorf("source worker %s authSecretRef namespace must not be empty", sourceKind)
	}

	return &SourceAuthSecretRef{
		Namespace: namespace,
		Name:      name,
	}, nil
}
