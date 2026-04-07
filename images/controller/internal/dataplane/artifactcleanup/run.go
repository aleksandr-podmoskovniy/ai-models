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

package artifactcleanup

import (
	"context"
	"errors"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Options struct {
	HandleJSON   string
	DryRun       bool
	Remover      modelpackports.Remover
	RegistryAuth modelpackports.RegistryAuth
}

func Run(ctx context.Context, options Options) error {
	handle, err := cleanuphandle.Decode(options.HandleJSON)
	if err != nil {
		return err
	}
	if handle.Kind != cleanuphandle.KindBackendArtifact || handle.Backend == nil {
		return errors.New("unsupported cleanup handle kind")
	}
	if options.DryRun {
		return nil
	}
	if options.Remover == nil {
		return errors.New("artifact cleanup remover must not be nil")
	}
	return options.Remover.Remove(ctx, handle.Backend.Reference, options.RegistryAuth)
}
