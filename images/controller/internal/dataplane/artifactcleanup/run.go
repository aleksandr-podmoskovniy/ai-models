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
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

type Options struct {
	HandleJSON     string
	DryRun         bool
	Remover        modelpackports.Remover
	StagingRemover uploadstagingports.Remover
	RegistryAuth   modelpackports.RegistryAuth
}

func Run(ctx context.Context, options Options) error {
	handle, err := cleanuphandle.Decode(options.HandleJSON)
	if err != nil {
		return err
	}
	if options.DryRun {
		return nil
	}
	switch handle.Kind {
	case cleanuphandle.KindBackendArtifact:
		if handle.Backend == nil {
			return errors.New("backend cleanup handle payload must not be empty")
		}
		if options.Remover == nil {
			return errors.New("artifact cleanup remover must not be nil")
		}
		return options.Remover.Remove(ctx, handle.Backend.Reference, options.RegistryAuth)
	case cleanuphandle.KindUploadStaging:
		if handle.UploadStaging == nil {
			return errors.New("upload staging cleanup handle payload must not be empty")
		}
		if options.StagingRemover == nil {
			return errors.New("upload staging cleanup remover must not be nil")
		}
		return options.StagingRemover.Delete(ctx, uploadstagingports.DeleteInput{
			Bucket: handle.UploadStaging.Bucket,
			Key:    handle.UploadStaging.Key,
		})
	default:
		return errors.New("unsupported cleanup handle kind")
	}
}
