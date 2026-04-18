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

package ingestadmission

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestValidateUploadSession(t *testing.T) {
	t.Parallel()

	session := UploadSession{
		Owner: OwnerBinding{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1",
			Namespace: "team-a",
			UID:       "1111-2222",
		},
		Identity: publicationdata.Identity{
			Scope:     publicationdata.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
	}

	t.Run("negative expected size fails", func(t *testing.T) {
		t.Parallel()
		session := session

		session.ExpectedSizeBytes = -1
		if err := ValidateUploadSession(session); err == nil {
			t.Fatal("expected negative expected size to fail")
		}
	})

	t.Run("invalid declared format fails", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = modelsv1alpha1.ModelInputFormat("Broken")
		if err := ValidateUploadSession(session); err == nil {
			t.Fatal("expected invalid format to fail")
		}
	})
}
