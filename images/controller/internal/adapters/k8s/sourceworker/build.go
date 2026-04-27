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

	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Build(
	request publicationports.Request,
	options Options,
	projectedAuthSecretName string,
	directUploadStateSecretName string,
) (*corev1.Pod, error) {
	sourcePlan, err := sourcePlan(request)
	if err != nil {
		return nil, err
	}
	return buildWithPlan(request, sourcePlan, options, projectedAuthSecretName, directUploadStateSecretName)
}

func buildWithPlan(
	request publicationports.Request,
	sourcePlan SourceWorkerPlan,
	options Options,
	projectedAuthSecretName string,
	directUploadStateSecretName string,
) (*corev1.Pod, error) {
	options = normalizeOptions(options)
	if err := validateOptions(sourcePlan, options); err != nil {
		return nil, err
	}
	if err := validateProjectedAuthSecretName(sourcePlan, projectedAuthSecretName); err != nil {
		return nil, err
	}

	name, err := resourcenames.SourceWorkerPodName(request.Owner.UID)
	if err != nil {
		return nil, err
	}
	artifactURI, err := publicationartifact.BuildOCIArtifactReference(options.OCIRepositoryPrefix, request.Identity, request.Owner.UID)
	if err != nil {
		return nil, err
	}

	container := buildContainer(request, sourcePlan, artifactURI, options, projectedAuthSecretName, directUploadStateSecretName)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   options.Namespace,
			Labels:      buildLabels(request.Owner),
			Annotations: resourcenames.OwnerAnnotations(request.Owner.Kind, request.Owner.Name, request.Owner.Namespace),
		},
		Spec: buildPodSpec(options, sourcePlan, container),
	}, nil
}

func validateProjectedAuthSecretName(
	plan SourceWorkerPlan,
	projectedAuthSecretName string,
) error {
	if !sourceAuthRequired(plan) {
		return nil
	}
	if strings.TrimSpace(projectedAuthSecretName) == "" {
		return errors.New("source worker projected auth secret name must not be empty when authSecretRef is set")
	}
	return nil
}

func sourceAuthRequired(plan SourceWorkerPlan) bool {
	return plan.HuggingFace != nil && plan.HuggingFace.AuthSecretRef != nil
}
