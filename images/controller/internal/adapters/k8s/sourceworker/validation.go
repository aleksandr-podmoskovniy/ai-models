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

	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	corev1 "k8s.io/api/core/v1"
)

type Options struct {
	Namespace               string
	Image                   string
	ServiceAccountName      string
	OCIRepositoryPrefix     string
	OCIInsecure             bool
	OCIRegistrySecretName   string
	OCIRegistryCASecretName string
	ImagePullPolicy         corev1.PullPolicy
}

func (o Options) Validate() error {
	switch {
	case strings.TrimSpace(o.Namespace) == "":
		return errors.New("source publish pod namespace must not be empty")
	case strings.TrimSpace(o.Image) == "":
		return errors.New("source publish pod image must not be empty")
	case strings.TrimSpace(o.ServiceAccountName) == "":
		return errors.New("source publish pod serviceAccountName must not be empty")
	case strings.TrimSpace(o.OCIRepositoryPrefix) == "":
		return errors.New("source publish pod OCI repository prefix must not be empty")
	case strings.TrimSpace(o.OCIRegistrySecretName) == "":
		return errors.New("source publish pod OCI registry secret name must not be empty")
	default:
		return nil
	}
}

func validateRequest(request publicationports.OperationContext) error {
	_, err := sourcePlan(request)
	return err
}

func sourcePlan(request publicationports.OperationContext) (publicationapp.SourceWorkerPlan, error) {
	if err := validateOwner(request.Request.Owner); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	if err := request.Request.Identity.Validate(); err != nil {
		return publicationapp.SourceWorkerPlan{}, err
	}
	if strings.TrimSpace(request.OperationName) == "" {
		return publicationapp.SourceWorkerPlan{}, errors.New("source publish pod operation name must not be empty")
	}
	if strings.TrimSpace(request.OperationNamespace) == "" {
		return publicationapp.SourceWorkerPlan{}, errors.New("source publish pod operation namespace must not be empty")
	}
	return publicationapp.PlanSourceWorker(request.Request.Spec, request.Request.Owner.Namespace)
}

func validateOwner(owner publicationports.Owner) error {
	switch {
	case strings.TrimSpace(string(owner.UID)) == "":
		return errors.New("source publish pod owner UID must not be empty")
	case strings.TrimSpace(owner.Kind) == "":
		return errors.New("source publish pod owner kind must not be empty")
	case strings.TrimSpace(owner.Name) == "":
		return errors.New("source publish pod owner name must not be empty")
	default:
		return nil
	}
}
