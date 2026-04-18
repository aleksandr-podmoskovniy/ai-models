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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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
				ObjectStorage: storageprojection.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
			},
		},
		PublicationRuntime: catalogstatus.Options{
			Runtime: sourceworker.RuntimeOptions{
				Namespace:             "d8-ai-models",
				Image:                 "backend:latest",
				ServiceAccountName:    "ai-models-controller",
				OCIRepositoryPrefix:   "registry.internal.local/ai-models",
				OCIRegistrySecretName: "ai-models-dmcr-auth-write",
				ObjectStorage: storageprojection.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("1"),
						corev1.ResourceMemory:           resource.MustParse("8Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("50Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("4"),
						corev1.ResourceMemory:           resource.MustParse("16Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("50Gi"),
					},
				},
			},
			MaxConcurrentWorkers: 1,
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
				ObjectStorage: storageprojection.Options{
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

func TestNewAcceptsWorkloadDeliveryWithDefaultInitContainerName(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := New(logger, Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:             "d8-ai-models",
				Image:                 "backend:latest",
				OCIRegistrySecretName: "ai-models-dmcr-auth-write",
				ObjectStorage: storageprojection.Options{
					Bucket:                "ai-models",
					EndpointURL:           "https://s3.example.com",
					Region:                "us-east-1",
					UsePathStyle:          true,
					CredentialsSecretName: "ai-models-artifacts",
				},
			},
		},
		WorkloadDelivery: workloaddelivery.Options{
			Service: modeldelivery.ServiceOptions{
				Render: modeldelivery.Options{
					RuntimeImage: "example.com/ai-models/controller-runtime:dev",
				},
				RegistrySourceNamespace:      "d8-ai-models",
				RegistrySourceAuthSecretName: "ai-models-dmcr-auth-read",
			},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
}

func TestManagerOptionsSetProductionControllerDefaults(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	scheme := k8sruntime.NewScheme()
	options := managerOptions(scheme, logger, RuntimeOptions{
		MetricsBindAddress:      ":18080",
		HealthProbeBindAddress:  ":18081",
		LeaderElection:          true,
		LeaderElectionID:        "ai-models-controller.deckhouse.io",
		LeaderElectionNamespace: "d8-ai-models",
	})

	if options.Scheme != scheme {
		t.Fatal("expected manager options to preserve provided scheme")
	}
	if got, want := options.Metrics.BindAddress, ":18080"; got != want {
		t.Fatalf("unexpected metrics bind address %q, want %q", got, want)
	}
	if got, want := options.HealthProbeBindAddress, ":18081"; got != want {
		t.Fatalf("unexpected health probe bind address %q, want %q", got, want)
	}
	if options.Logger.GetSink() == nil {
		t.Fatal("expected manager logger to be configured")
	}
	if options.Controller.Logger.GetSink() == nil {
		t.Fatal("expected controller logger defaults to be configured")
	}
	if got, want := options.Controller.CacheSyncTimeout, defaultControllerCacheSyncTimeout; got != want {
		t.Fatalf("unexpected controller cache sync timeout %s, want %s", got, want)
	}
	if options.Controller.RecoverPanic == nil || !*options.Controller.RecoverPanic {
		t.Fatal("expected controller recover panic default to be enabled")
	}
	if options.Controller.UsePriorityQueue == nil || !*options.Controller.UsePriorityQueue {
		t.Fatal("expected controller priority queue default to be enabled")
	}
}
