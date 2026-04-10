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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
)

const (
	serviceLabelKey = "ai-models.deckhouse.io/upload-service"
	uploadPort      = 8444
	defaultTokenTTL = 30 * time.Minute
)

type Options struct {
	Runtime  workloadpod.RuntimeOptions
	Ingress  IngressOptions
	TokenTTL time.Duration
}

type IngressOptions struct {
	Host          string
	ClassName     string
	TLSSecretName string
}

func normalizeOptions(options Options) Options {
	options.Runtime = workloadpod.NormalizeRuntimeOptions(options.Runtime)
	options.Ingress = normalizeIngressOptions(options.Ingress)
	if options.TokenTTL <= 0 {
		options.TokenTTL = defaultTokenTTL
	}

	return options
}

func normalizeIngressOptions(options IngressOptions) IngressOptions {
	options.Host = strings.TrimSpace(options.Host)
	options.ClassName = strings.TrimSpace(options.ClassName)
	options.TLSSecretName = strings.TrimSpace(options.TLSSecretName)
	return options
}

func (o IngressOptions) Enabled() bool {
	return strings.TrimSpace(o.Host) != ""
}

func (o Options) Validate() error {
	if err := workloadpod.ValidateRuntimeOptions("upload session", o.Runtime); err != nil {
		return err
	}
	if err := objectstorage.ValidateOptions("upload session", o.Runtime.ObjectStorage); err != nil {
		return err
	}
	if o.TokenTTL <= 0 {
		return errors.New("upload session token ttl must be positive")
	}

	return nil
}
