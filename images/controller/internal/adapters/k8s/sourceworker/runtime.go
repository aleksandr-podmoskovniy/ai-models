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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageaccounting"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
)

type RuntimeOptions struct {
	Namespace               string
	Image                   string
	ImagePullSecretName     string
	ServiceAccountName      string
	OCIRepositoryPrefix     string
	OCIInsecure             bool
	OCIRegistrySecretName   string
	OCIRegistryCASecretName string
	OCIDirectUploadEndpoint string
	ObjectStorage           storageprojection.Options
	StorageAccounting       storageaccounting.Options
	SourceFetch             publicationports.SourceFetchMode
	ImagePullPolicy         corev1.PullPolicy
	Resources               corev1.ResourceRequirements
}

func NormalizeRuntimeOptions(options RuntimeOptions) RuntimeOptions {
	if options.ImagePullPolicy == "" {
		options.ImagePullPolicy = corev1.PullIfNotPresent
	}
	options.SourceFetch = publicationports.NormalizeSourceFetchMode(options.SourceFetch)
	return options
}

func ValidateRuntimeOptions(component string, options RuntimeOptions) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return errors.New("source worker runtime component name must not be empty")
	}
	if err := publicationports.ValidateSourceFetchMode(options.SourceFetch); err != nil {
		return fmt.Errorf("%s %w", component, err)
	}
	if err := options.StorageAccounting.Validate(); err != nil {
		return fmt.Errorf("%s %w", component, err)
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
	case strings.TrimSpace(options.OCIDirectUploadEndpoint) == "":
		return fmt.Errorf("%s OCI direct upload endpoint must not be empty", component)
	default:
		return validateRuntimeResources(component, options.Resources)
	}
}

func validateRuntimeResources(component string, requirements corev1.ResourceRequirements) error {
	for _, entry := range []struct {
		listName string
		list     corev1.ResourceList
		key      corev1.ResourceName
	}{
		{listName: "requests", list: requirements.Requests, key: corev1.ResourceCPU},
		{listName: "requests", list: requirements.Requests, key: corev1.ResourceMemory},
		{listName: "requests", list: requirements.Requests, key: corev1.ResourceEphemeralStorage},
		{listName: "limits", list: requirements.Limits, key: corev1.ResourceCPU},
		{listName: "limits", list: requirements.Limits, key: corev1.ResourceMemory},
		{listName: "limits", list: requirements.Limits, key: corev1.ResourceEphemeralStorage},
	} {
		if err := validatePositiveResourceQuantity(component, entry.listName, entry.list, entry.key); err != nil {
			return err
		}
	}
	return nil
}

func validatePositiveResourceQuantity(component, listName string, list corev1.ResourceList, key corev1.ResourceName) error {
	quantity, ok := list[key]
	if !ok {
		return fmt.Errorf("%s resource %s.%s must be set", component, listName, key)
	}
	if quantity.Sign() <= 0 {
		return fmt.Errorf("%s resource %s.%s must be greater than zero", component, listName, key)
	}
	return nil
}

func terminationMessage(pod *corev1.Pod, containerName string) string {
	if pod == nil {
		return ""
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != containerName || status.State.Terminated == nil {
			continue
		}
		return strings.TrimSpace(status.State.Terminated.Message)
	}
	return ""
}
