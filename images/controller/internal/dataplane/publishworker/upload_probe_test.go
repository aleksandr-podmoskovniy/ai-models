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

package publishworker

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestPublishFromUploadRejectsDirectSafetensorsBeforeWorkspace(t *testing.T) {
	t.Parallel()

	modelPath := writeTempFile(t, "model.safetensors", []byte("header"))
	publisher := fakePublisher{
		onPublish: func(modelpackports.PublishInput) error {
			t.Fatal("publisher must not be called for invalid direct safetensors upload")
			return nil
		},
	}

	_, err := run(context.Background(), Options{
		SourceType:         modelsv1alpha1.ModelSourceTypeUpload,
		ArtifactURI:        "registry.example.com/ai-models/catalog/model:published",
		UploadPath:         modelPath,
		ModelPackPublisher: publisher,
	})
	if err == nil {
		t.Fatal("expected invalid direct safetensors upload to fail")
	}
}
