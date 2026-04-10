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

package bootstrap

import (
	"io"
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
)

func TestNewWiresPublicationRuntimeForOCIArtifactPlane(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(logger, Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:             "d8-ai-models",
				Image:                 "backend:latest",
				OCIRegistrySecretName: "ai-models-dmcr-auth-write",
				ObjectStorage: objectstorage.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
			},
		},
		PublicationRuntime: catalogstatus.Options{
			Runtime: catalogstatus.PublicationRuntimeOptions{
				Namespace:             "d8-ai-models",
				Image:                 "backend:latest",
				ServiceAccountName:    "ai-models-controller",
				OCIRepositoryPrefix:   "registry.internal.local/ai-models",
				OCIRegistrySecretName: "ai-models-dmcr-auth-write",
				ObjectStorage: objectstorage.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !application.publicationRuntime.Enabled() {
		t.Fatal("expected publication runtime to be configured")
	}
}

func TestNewAllowsCleanupOnlyRuntimeWithoutPublicationPlaneConfiguration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(logger, Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:             "d8-ai-models",
				Image:                 "backend:latest",
				OCIRegistrySecretName: "ai-models-dmcr-auth-write",
				ObjectStorage: objectstorage.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if application.publicationRuntime.Enabled() {
		t.Fatal("expected publication runtime to stay disabled")
	}
}
