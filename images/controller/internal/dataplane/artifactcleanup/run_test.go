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
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type fakeRemover struct {
	reference string
}

func (f *fakeRemover) Remove(_ context.Context, reference string, _ modelpackports.RegistryAuth) error {
	f.reference = reference
	return nil
}

func TestRunInvokesRemover(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{}
	err := Run(context.Background(), Options{
		HandleJSON: `{"kind":"BackendArtifact","backend":{"reference":"registry.example.com/model@sha256:deadbeef"}}`,
		Remover:    remover,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := remover.reference, "registry.example.com/model@sha256:deadbeef"; got != want {
		t.Fatalf("unexpected reference %q", got)
	}
}
