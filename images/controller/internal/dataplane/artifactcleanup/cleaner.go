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

type ObjectStorageRemover interface {
	uploadstagingports.Remover
	uploadstagingports.PrefixRemover
}

type ObjectStorageFactory func() (ObjectStorageRemover, error)

type Cleaner struct {
	Remover             modelpackports.Remover
	ObjectStorage       ObjectStorageFactory
	ObjectStorageBucket string
	RegistryAuth        modelpackports.RegistryAuth
}

func (c Cleaner) Cleanup(ctx context.Context, handle cleanuphandle.Handle) error {
	if err := handle.Validate(); err != nil {
		return err
	}
	encoded, err := cleanuphandle.Encode(handle)
	if err != nil {
		return err
	}

	var objectStorage ObjectStorageRemover
	if needsObjectStorage(handle.Kind) {
		if c.ObjectStorage == nil {
			return errors.New("artifact cleanup object storage factory must not be nil")
		}
		objectStorage, err = c.ObjectStorage()
		if err != nil {
			return err
		}
	}

	return Run(ctx, Options{
		HandleJSON:          encoded,
		Remover:             c.Remover,
		StagingRemover:      objectStorage,
		PrefixRemover:       objectStorage,
		ObjectStorageBucket: c.ObjectStorageBucket,
		RegistryAuth:        c.RegistryAuth,
	})
}

func needsObjectStorage(kind cleanuphandle.Kind) bool {
	return kind == cleanuphandle.KindBackendArtifact || kind == cleanuphandle.KindUploadStaging
}
