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
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
)

func TestBootstrapOptionsEnableWorkloadDelivery(t *testing.T) {
	t.Parallel()

	config := managerConfig{
		LogFormat:                          "json",
		LogLevel:                           "debug",
		CleanupJobImage:                    "example.com/controller-runtime:dev",
		CleanupJobNamespace:                "d8-ai-models",
		PublicationWorkerNamespace:         "d8-ai-models",
		PublicationOCICASecretName:         "ai-models-dmcr-ca",
		PublicationOCIInsecure:             false,
		PublicationOCIDirectUploadEndpoint: "https://ai-models-dmcr.d8-ai-models.svc.cluster.local:5443",
		PublicationSourceFetchMode:         publicationports.SourceFetchModeDirect,
		ArtifactsBucket:                    "models",
		ArtifactsS3Endpoint:                "https://s3.example.test",
		ArtifactsS3Region:                  "test",
		ArtifactsCredentialsSecretName:     "artifacts-credentials",
		NodeCacheEnabled:                   true,
		NodeCacheMaxSize:                   "200Gi",
		NodeCacheFallbackVolumeSize:        "32Gi",
		NodeCacheStorageClassName:          "ai-models-node-cache",
		NodeCacheVolumeGroupSetName:        "ai-models-node-cache",
		NodeCacheVolumeGroupNameOnNode:     "ai-models-cache",
		NodeCacheThinPoolName:              "model-cache",
		NodeCacheNodeSelectorJSON:          `{"node-role.kubernetes.io/worker":""}`,
		NodeCacheBlockDeviceJSON:           `{"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
	}

	options := config.bootstrapOptions(corev1.ResourceRequirements{})

	if got, want := options.WorkloadDelivery.Service.Render.RuntimeImage, config.CleanupJobImage; got != want {
		t.Fatalf("delivery runtime image = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.Render.LogLevel, config.LogLevel; got != want {
		t.Fatalf("delivery runtime log level = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.Render.CacheMountPath, modeldelivery.DefaultCacheMountPath; got != want {
		t.Fatalf("delivery cache mount path = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.ManagedCache.Enabled, config.NodeCacheEnabled; got != want {
		t.Fatalf("delivery managed cache enabled = %t, want %t", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.ManagedCache.StorageClassName, config.NodeCacheStorageClassName; got != want {
		t.Fatalf("delivery managed cache storage class name = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.ManagedCache.VolumeSize, config.NodeCacheFallbackVolumeSize; got != want {
		t.Fatalf("delivery managed cache volume size = %q, want %q", got, want)
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
	if got, want := options.PublicationRuntime.Runtime.SourceFetch, config.PublicationSourceFetchMode; got != want {
		t.Fatalf("publication runtime source fetch mode = %q, want %q", got, want)
	}
	if got, want := options.PublicationRuntime.Runtime.OCIDirectUploadEndpoint, config.PublicationOCIDirectUploadEndpoint; got != want {
		t.Fatalf("publication runtime OCI direct upload endpoint = %q, want %q", got, want)
	}
	if got, want := options.NodeCacheSubstrate.MaxSize, config.NodeCacheMaxSize; got != want {
		t.Fatalf("node cache max size = %q, want %q", got, want)
	}
	if got, want := options.NodeCacheSubstrate.StorageClassName, config.NodeCacheStorageClassName; got != want {
		t.Fatalf("node cache storage class name = %q, want %q", got, want)
	}
	if got, want := options.NodeCacheSubstrate.ThinPoolName, config.NodeCacheThinPoolName; got != want {
		t.Fatalf("node cache thin pool name = %q, want %q", got, want)
	}
}

func TestParseManagerConfigRejectsInvalidNodeCacheSelectors(t *testing.T) {
	t.Parallel()

	_, exitCode, err := parseManagerConfig([]string{
		"--node-cache-node-selector-json={",
	})
	if err == nil {
		t.Fatal("parseManagerConfig() error = nil, want error")
	}
	if exitCode != 2 {
		t.Fatalf("parseManagerConfig() exitCode = %d, want 2", exitCode)
	}
}
