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

package workloaddelivery

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
)

const (
	defaultControllerNamePrefix = "workloaddelivery"
)

type Options struct {
	Service modeldelivery.ServiceOptions
}

func (o Options) Enabled() bool {
	return strings.TrimSpace(o.Service.Render.RuntimeImage) != "" &&
		strings.TrimSpace(o.Service.RegistrySourceNamespace) != "" &&
		strings.TrimSpace(o.Service.RegistrySourceAuthSecretName) != ""
}

func (o Options) Validate() error {
	if !o.Enabled() {
		return nil
	}
	if err := modeldelivery.ValidateOptions(o.Service.Render); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(o.Service.RegistrySourceNamespace) == "":
		return errors.New("workload delivery registry source namespace must not be empty")
	case strings.TrimSpace(o.Service.RegistrySourceAuthSecretName) == "":
		return errors.New("workload delivery registry source auth secret name must not be empty")
	}
	return nil
}

func normalizeOptions(options Options) Options {
	options.Service = modeldelivery.ServiceOptions{
		Render:                       modeldelivery.NormalizeOptions(options.Service.Render),
		RegistrySourceNamespace:      options.Service.RegistrySourceNamespace,
		RegistrySourceAuthSecretName: options.Service.RegistrySourceAuthSecretName,
		RegistrySourceCASecretName:   options.Service.RegistrySourceCASecretName,
	}
	return options
}

func controllerLogger(kind string) *slog.Logger {
	return slog.Default().With(
		slog.String("controller", "workloaddelivery"),
		slog.String("workloadKind", kind),
	)
}
