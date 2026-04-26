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

package catalogcleanup

import (
	"context"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ArtifactCleaner interface {
	Cleanup(ctx context.Context, handle cleanuphandle.Handle) error
}

type CleanupOptions struct {
	Namespace string
	Cleaner   ArtifactCleaner
}

type cleanupOwner struct {
	UID       types.UID
	Kind      string
	Name      string
	Namespace string
}

func (o CleanupOptions) Validate() error {
	if strings.TrimSpace(o.Namespace) == "" {
		return errors.New("cleanup namespace must not be empty")
	}
	if o.Cleaner == nil {
		return errors.New("cleanup cleaner must not be nil")
	}
	return nil
}

func cleanupOwnerFor(object client.Object) (cleanupOwner, error) {
	kind, err := modelobject.KindFor(object)
	if err != nil {
		return cleanupOwner{}, err
	}
	return cleanupOwner{
		UID:       object.GetUID(),
		Kind:      kind,
		Name:      object.GetName(),
		Namespace: object.GetNamespace(),
	}, nil
}
