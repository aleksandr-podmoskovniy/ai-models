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

package uploadsession

import (
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	serviceLabelKey = "ai-models.deckhouse.io/upload-service"
	uploadPort      = 8444
	defaultTokenTTL = 30 * time.Minute
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
	TokenTTL                time.Duration
}

func normalizeOptions(options Options) Options {
	if options.ImagePullPolicy == "" {
		options.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if options.TokenTTL <= 0 {
		options.TokenTTL = defaultTokenTTL
	}

	return options
}

func (o Options) Validate() error {
	if strings.TrimSpace(o.Namespace) == "" {
		return errors.New("upload session namespace must not be empty")
	}
	if strings.TrimSpace(o.Image) == "" {
		return errors.New("upload session image must not be empty")
	}
	if strings.TrimSpace(o.ServiceAccountName) == "" {
		return errors.New("upload session serviceAccountName must not be empty")
	}
	if strings.TrimSpace(o.OCIRepositoryPrefix) == "" {
		return errors.New("upload session OCI repository prefix must not be empty")
	}
	if strings.TrimSpace(o.OCIRegistrySecretName) == "" {
		return errors.New("upload session OCI registry secret name must not be empty")
	}
	if o.TokenTTL <= 0 {
		return errors.New("upload session token ttl must be positive")
	}

	return nil
}
