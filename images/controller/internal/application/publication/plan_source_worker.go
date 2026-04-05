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

package publication

import (
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type SourceWorkerPlan struct {
	SourceType  modelsv1alpha1.ModelSourceType
	Task        string
	HuggingFace *HuggingFaceSourcePlan
	HTTP        *HTTPSourcePlan
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

type HTTPSourcePlan struct {
	URL           string
	CABundle      []byte
	AuthSecretRef *SourceAuthSecretRef
}

func PlanSourceWorker(spec modelsv1alpha1.ModelSpec, ownerNamespace string) (SourceWorkerPlan, error) {
	plan := SourceWorkerPlan{
		SourceType: spec.Source.Type,
	}
	if spec.RuntimeHints != nil {
		plan.Task = strings.TrimSpace(spec.RuntimeHints.Task)
	}

	switch spec.Source.Type {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		if spec.Source.HuggingFace == nil {
			return SourceWorkerPlan{}, errors.New("source publish pod huggingFace source must not be empty")
		}
		authSecretRef, err := resolveSourceAuthSecretRef(
			spec.Source.HuggingFace.AuthSecretRef,
			ownerNamespace,
			"huggingFace",
		)
		if err != nil {
			return SourceWorkerPlan{}, err
		}
		plan.HuggingFace = &HuggingFaceSourcePlan{
			RepoID:        spec.Source.HuggingFace.RepoID,
			Revision:      spec.Source.HuggingFace.Revision,
			AuthSecretRef: authSecretRef,
		}
		return plan, nil
	case modelsv1alpha1.ModelSourceTypeHTTP:
		if spec.Source.HTTP == nil {
			return SourceWorkerPlan{}, errors.New("source publish pod http source must not be empty")
		}
		if strings.TrimSpace(spec.Source.HTTP.URL) == "" {
			return SourceWorkerPlan{}, errors.New("source publish pod http url must not be empty")
		}
		authSecretRef, err := resolveSourceAuthSecretRef(
			spec.Source.HTTP.AuthSecretRef,
			ownerNamespace,
			"http",
		)
		if err != nil {
			return SourceWorkerPlan{}, err
		}
		if plan.Task == "" {
			return SourceWorkerPlan{}, errors.New("source publish pod http source requires spec.runtimeHints.task")
		}
		plan.HTTP = &HTTPSourcePlan{
			URL:           spec.Source.HTTP.URL,
			CABundle:      spec.Source.HTTP.CABundle,
			AuthSecretRef: authSecretRef,
		}
		return plan, nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		return SourceWorkerPlan{}, errors.New("source publish pod upload source must be implemented as a session, not a batch-style worker pod")
	default:
		return SourceWorkerPlan{}, fmt.Errorf("source publish pod does not support source type %q", spec.Source.Type)
	}
}

func resolveSourceAuthSecretRef(
	ref *modelsv1alpha1.SecretReference,
	ownerNamespace string,
	sourceKind string,
) (*SourceAuthSecretRef, error) {
	if ref == nil {
		return nil, nil
	}

	name := strings.TrimSpace(ref.Name)
	if name == "" {
		return nil, fmt.Errorf("source publish pod %s authSecretRef name must not be empty", sourceKind)
	}

	namespace := strings.TrimSpace(ref.Namespace)
	resolvedOwnerNamespace := strings.TrimSpace(ownerNamespace)
	if resolvedOwnerNamespace != "" {
		switch {
		case namespace == "":
			namespace = resolvedOwnerNamespace
		case namespace != resolvedOwnerNamespace:
			return nil, fmt.Errorf(
				"source publish pod %s authSecretRef namespace must match owner namespace %q",
				sourceKind,
				resolvedOwnerNamespace,
			)
		}
	} else if namespace == "" {
		namespace = resolvedOwnerNamespace
	}
	if namespace == "" {
		return nil, fmt.Errorf("source publish pod %s authSecretRef namespace must not be empty", sourceKind)
	}

	return &SourceAuthSecretRef{
		Namespace: namespace,
		Name:      name,
	}, nil
}
