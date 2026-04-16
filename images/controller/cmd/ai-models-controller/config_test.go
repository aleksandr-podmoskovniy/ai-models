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

package main

import (
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestBootstrapOptionsEnableWorkloadDelivery(t *testing.T) {
	t.Parallel()

	config := managerConfig{
		LogFormat:                      "json",
		LogLevel:                       "debug",
		CleanupJobImage:                "example.com/controller-runtime:dev",
		CleanupJobNamespace:            "d8-ai-models",
		PublicationWorkerNamespace:     "d8-ai-models",
		PublicationOCICASecretName:     "ai-models-dmcr-ca",
		PublicationOCIInsecure:         false,
		ArtifactsBucket:                "models",
		ArtifactsS3Endpoint:            "https://s3.example.test",
		ArtifactsS3Region:              "test",
		ArtifactsCredentialsSecretName: "artifacts-credentials",
	}

	options := config.bootstrapOptions(
		workloadpod.WorkVolumeTypeEmptyDir,
		resource.MustParse("50Gi"),
		corev1.ResourceRequirements{},
	)

	if got, want := options.WorkloadDelivery.Service.Render.RuntimeImage, config.CleanupJobImage; got != want {
		t.Fatalf("delivery runtime image = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.Render.LogLevel, config.LogLevel; got != want {
		t.Fatalf("delivery runtime log level = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.Render.CacheMountPath, modeldelivery.DefaultCacheMountPath; got != want {
		t.Fatalf("delivery cache mount path = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.RegistrySourceNamespace, config.PublicationWorkerNamespace; got != want {
		t.Fatalf("delivery source namespace = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.RegistrySourceAuthSecretName, defaultDMCRReadAuthSecretName; got != want {
		t.Fatalf("delivery source auth secret = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.RegistrySourceCASecretName, config.PublicationOCICASecretName; got != want {
		t.Fatalf("delivery source CA secret = %q, want %q", got, want)
	}
	if got, want := options.PublicationRuntime.RuntimeLogLevel, config.LogLevel; got != want {
		t.Fatalf("publication runtime log level = %q, want %q", got, want)
	}
}
