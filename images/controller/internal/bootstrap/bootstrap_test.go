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
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

func TestNewWiresPublicationRuntimeForOCIArtifactPlane(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(logger, Options{
		Cleanup: catalogcleanup.Options{
			Cleanup: catalogcleanup.CleanupOptions{
				Namespace: "d8-ai-models",
				Cleaner:   fakeBootstrapCleaner{},
			},
		},
		PublicationRuntime: catalogstatus.Options{
			Runtime: sourceworker.RuntimeOptions{
				Namespace:               "d8-ai-models",
				Image:                   "backend:latest",
				ServiceAccountName:      "ai-models-controller",
				OCIRepositoryPrefix:     "registry.internal.local/ai-models",
				OCIRegistrySecretName:   "ai-models-dmcr-auth-write",
				OCIDirectUploadEndpoint: "https://ai-models-dmcr.d8-ai-models.svc.cluster.local:5443",
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
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:              resource.MustParse("4"),
						corev1.ResourceMemory:           resource.MustParse("16Gi"),
						corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
					},
				},
			},
			MaxConcurrentWorkers: 1,
			UploadStagingBucket:  "ai-models",
			UploadStagingClient:  fakeBootstrapMultipartStager{},
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if !application.publicationRuntime.Enabled() {
		t.Fatal("expected publication runtime to be configured")
	}
}

type fakeBootstrapMultipartStager struct{}

type fakeBootstrapCleaner struct{}

func (fakeBootstrapCleaner) Cleanup(context.Context, cleanuphandle.Handle) error {
	return nil
}

func (fakeBootstrapMultipartStager) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (fakeBootstrapMultipartStager) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (fakeBootstrapMultipartStager) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (fakeBootstrapMultipartStager) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (fakeBootstrapMultipartStager) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (fakeBootstrapMultipartStager) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{}, nil
}

func TestNewAllowsCleanupOnlyRuntimeWithoutPublicationPlaneConfiguration(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(logger, Options{
		Cleanup: catalogcleanup.Options{
			Cleanup: catalogcleanup.CleanupOptions{
				Namespace: "d8-ai-models",
				Cleaner:   fakeBootstrapCleaner{},
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

func TestNewAcceptsWorkloadDeliveryWithDefaultLegacyInitCleanupName(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := New(logger, Options{
		Cleanup: catalogcleanup.Options{
			Cleanup: catalogcleanup.CleanupOptions{
				Namespace: "d8-ai-models",
				Cleaner:   fakeBootstrapCleaner{},
			},
		},
		WorkloadDelivery: workloaddelivery.Options{
			Service: modeldelivery.ServiceOptions{
				RegistrySourceNamespace: "d8-ai-models",
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
