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

package workloadpod

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	corev1 "k8s.io/api/core/v1"
)

type RuntimeOptions struct {
	Namespace               string
	Image                   string
	ServiceAccountName      string
	OCIRepositoryPrefix     string
	OCIInsecure             bool
	OCIRegistrySecretName   string
	OCIRegistryCASecretName string
	ObjectStorage           objectstorage.Options
	ImagePullPolicy         corev1.PullPolicy
}

func NormalizeRuntimeOptions(options RuntimeOptions) RuntimeOptions {
	if options.ImagePullPolicy == "" {
		options.ImagePullPolicy = corev1.PullIfNotPresent
	}
	return options
}

func ValidateRuntimeOptions(component string, options RuntimeOptions) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return errors.New("workload pod runtime component name must not be empty")
	}

	switch {
	case strings.TrimSpace(options.Namespace) == "":
		return fmt.Errorf("%s namespace must not be empty", component)
	case strings.TrimSpace(options.Image) == "":
		return fmt.Errorf("%s image must not be empty", component)
	case strings.TrimSpace(options.ServiceAccountName) == "":
		return fmt.Errorf("%s serviceAccountName must not be empty", component)
	case strings.TrimSpace(options.OCIRepositoryPrefix) == "":
		return fmt.Errorf("%s OCI repository prefix must not be empty", component)
	case strings.TrimSpace(options.OCIRegistrySecretName) == "":
		return fmt.Errorf("%s OCI registry secret name must not be empty", component)
	default:
		return nil
	}
}
