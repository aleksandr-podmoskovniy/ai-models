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
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestDefaultManagerConfigPublicationRuntimeDefaults(t *testing.T) {
	t.Setenv(publicationMaxConcurrentWorkersEnv, "")
	t.Setenv(publicationWorkerMemoryRequestEnv, "")
	t.Setenv(publicationWorkerMemoryLimitEnv, "")
	t.Setenv(uploadServiceNameEnv, "")

	config := defaultManagerConfig()
	if got, want := config.PublicationMaxConcurrentWorkers, 4; got != want {
		t.Fatalf("PublicationMaxConcurrentWorkers = %d, want %d", got, want)
	}
	if got, want := config.PublicationWorkerMemoryRequest, "1Gi"; got != want {
		t.Fatalf("PublicationWorkerMemoryRequest = %q, want %q", got, want)
	}
	if got, want := config.PublicationWorkerMemoryLimit, "2Gi"; got != want {
		t.Fatalf("PublicationWorkerMemoryLimit = %q, want %q", got, want)
	}
	if got, want := config.UploadServiceName, defaultUploadGatewayServiceName; got != want {
		t.Fatalf("UploadServiceName = %q, want %q", got, want)
	}

	resources, resourceErr := runtimeResources(config)
	if resourceErr != nil {
		t.Fatalf("runtimeResources() error = %v", resourceErr.cause)
	}
	if got, want := resources.Requests[corev1.ResourceMemory], resource.MustParse("1Gi"); got.Cmp(want) != 0 {
		t.Fatalf("memory request = %s, want %s", got.String(), want.String())
	}
	if got, want := resources.Limits[corev1.ResourceMemory], resource.MustParse("2Gi"); got.Cmp(want) != 0 {
		t.Fatalf("memory limit = %s, want %s", got.String(), want.String())
	}
}

func TestBootstrapOptionsEnableWorkloadDelivery(t *testing.T) {
	t.Parallel()

	config := managerConfig{
		LogFormat:                                  "json",
		LogLevel:                                   "debug",
		CleanupNamespace:                           "d8-ai-models",
		PublicationWorkerImage:                     "example.com/controller-runtime:dev",
		PublicationWorkerImagePullSecretName:       "module-registry",
		PublicationWorkerNamespace:                 "d8-ai-models",
		WorkloadDeliveryRuntimeImagePullSecretName: "workload-runtime-pull",
		PublicationOCICASecretName:                 "ai-models-dmcr-ca",
		PublicationOCIInsecure:                     false,
		PublicationOCIDirectUploadEndpoint:         "https://ai-models-dmcr.d8-ai-models.svc.cluster.local:5443",
		PublicationSourceFetchMode:                 publicationports.SourceFetchModeDirect,
		ArtifactsBucket:                            "models",
		ArtifactsS3Endpoint:                        "https://s3.example.test",
		ArtifactsS3Region:                          "test",
		ArtifactsCredentialsSecretName:             "artifacts-credentials",
		NodeCacheEnabled:                           true,
		NodeCacheRuntimeImage:                      "example.com/node-cache-runtime:dev",
		NodeCacheCSIRegistrarImage:                 "registry.example.test/csi-node-driver-registrar@sha256:aaaa",
		NodeCacheMaxSize:                           "200Gi",
		NodeCacheSharedVolumeSize:                  "64Gi",
		NodeCacheStorageClassName:                  "ai-models-node-cache",
		NodeCacheVolumeGroupSetName:                "ai-models-node-cache",
		NodeCacheVolumeGroupNameOnNode:             "ai-models-cache",
		NodeCacheThinPoolName:                      "model-cache",
		NodeCacheNodeSelectorJSON:                  `{"node-role.kubernetes.io/worker":""}`,
		NodeCacheBlockDeviceJSON:                   `{"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
	}

	options := config.bootstrapOptions(corev1.ResourceRequirements{})

	if got, want := options.WorkloadDelivery.Service.Render.RuntimeImage, config.PublicationWorkerImage; got != want {
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
	if got, want := options.WorkloadDelivery.Service.ManagedCache.NodeSelector["node-role.kubernetes.io/worker"], ""; got != want {
		t.Fatalf("delivery managed cache node selector = %q, want %q", got, want)
	}
	if got, want := options.WorkloadDelivery.Service.ManagedCache.NodeSelector["ai.deckhouse.io/node-cache-runtime-ready"], "true"; got != want {
		t.Fatalf("delivery managed cache ready selector = %q, want %q", got, want)
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
	if got, want := options.WorkloadDelivery.Service.RuntimeImagePullSecretName, config.WorkloadDeliveryRuntimeImagePullSecretName; got != want {
		t.Fatalf("delivery runtime image pull secret = %q, want %q", got, want)
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
	if got, want := options.NodeCacheRuntime.CSIRegistrarImage, config.NodeCacheCSIRegistrarImage; got != want {
		t.Fatalf("node cache CSI registrar image = %q, want %q", got, want)
	}
	if got, want := options.NodeCacheRuntime.RuntimeImage, config.NodeCacheRuntimeImage; got != want {
		t.Fatalf("node cache runtime image = %q, want %q", got, want)
	}
	if got, want := options.NodeCacheRuntime.MaxTotalSize, config.NodeCacheSharedVolumeSize; got != want {
		t.Fatalf("node cache runtime max total size = %q, want %q", got, want)
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

func TestParseManagerConfigRejectsEmptyCSIRegistrarImageWhenNodeCacheEnabled(t *testing.T) {
	t.Parallel()

	_, exitCode, err := parseManagerConfig([]string{
		"--node-cache-enabled=true",
		"--node-cache-runtime-image=example.com/node-cache-runtime:dev",
		`--node-cache-node-selector-json={"node-role.kubernetes.io/worker":""}`,
		`--node-cache-block-device-selector-json={"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
	})
	if err == nil {
		t.Fatal("parseManagerConfig() error = nil, want error")
	}
	if exitCode != 2 {
		t.Fatalf("parseManagerConfig() exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(err.Error(), "node-cache-csi-registrar-image must not be empty when node cache is enabled") {
		t.Fatalf("parseManagerConfig() error = %q", err.Error())
	}
}

func TestParseManagerConfigRejectsEmptyNodeCacheRuntimeImageWhenNodeCacheEnabled(t *testing.T) {
	t.Parallel()

	_, exitCode, err := parseManagerConfig([]string{
		"--node-cache-enabled=true",
		"--node-cache-csi-registrar-image=registry.example.test/csi-node-driver-registrar@sha256:aaaa",
		`--node-cache-node-selector-json={"node-role.kubernetes.io/worker":""}`,
		`--node-cache-block-device-selector-json={"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
	})
	if err == nil {
		t.Fatal("parseManagerConfig() error = nil, want error")
	}
	if exitCode != 2 {
		t.Fatalf("parseManagerConfig() exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(err.Error(), "node-cache-runtime-image must not be empty when node cache is enabled") {
		t.Fatalf("parseManagerConfig() error = %q", err.Error())
	}
}

func TestParseManagerConfigRejectsNodeCacheSharedVolumeGreaterThanMaxSize(t *testing.T) {
	t.Parallel()

	_, exitCode, err := parseManagerConfig([]string{
		"--node-cache-enabled=true",
		"--node-cache-runtime-image=example.com/node-cache-runtime:dev",
		"--node-cache-csi-registrar-image=registry.example.test/csi-node-driver-registrar@sha256:aaaa",
		"--node-cache-max-size=64Gi",
		"--node-cache-shared-volume-size=65Gi",
		`--node-cache-node-selector-json={"node-role.kubernetes.io/worker":""}`,
		`--node-cache-block-device-selector-json={"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
	})
	if err == nil {
		t.Fatal("parseManagerConfig() error = nil, want error")
	}
	if exitCode != 2 {
		t.Fatalf("parseManagerConfig() exitCode = %d, want 2", exitCode)
	}
	if !strings.Contains(err.Error(), "node-cache-shared-volume-size must not exceed node-cache-max-size") {
		t.Fatalf("parseManagerConfig() error = %q", err.Error())
	}
}

func TestParseManagerConfigRejectsEmptyNodeCacheSelectorsWhenEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "default selectors",
			args:    []string{"--node-cache-enabled=true"},
			wantErr: "node-cache-node-selector-json must not be empty when node cache is enabled",
		},
		{
			name: "empty node selector",
			args: []string{
				"--node-cache-enabled=true",
				"--node-cache-node-selector-json={}",
				`--node-cache-block-device-selector-json={"status.blockdevice.storage.deckhouse.io/model":"nvme"}`,
			},
			wantErr: "node-cache-node-selector-json must not be empty when node cache is enabled",
		},
		{
			name: "empty block device selector",
			args: []string{
				"--node-cache-enabled=true",
				`--node-cache-node-selector-json={"node-role.kubernetes.io/worker":""}`,
				"--node-cache-block-device-selector-json={}",
			},
			wantErr: "node-cache-block-device-selector-json must not be empty when node cache is enabled",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, exitCode, err := parseManagerConfig(tt.args)
			if err == nil {
				t.Fatal("parseManagerConfig() error = nil, want error")
			}
			if exitCode != 2 {
				t.Fatalf("parseManagerConfig() exitCode = %d, want 2", exitCode)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseManagerConfig() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseManagerConfigAcceptsQuotedNodeCacheSelectors(t *testing.T) {
	t.Parallel()

	config, exitCode, err := parseManagerConfig([]string{
		`--node-cache-node-selector-json="{\"node-role.kubernetes.io/worker\":\"\"}"`,
		`--node-cache-block-device-selector-json="{\"status.blockdevice.storage.deckhouse.io/model\":\"nvme\"}"`,
	})
	if err != nil {
		t.Fatalf("parseManagerConfig() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("parseManagerConfig() exitCode = %d, want 0", exitCode)
	}
	if got, want := config.NodeCacheNodeSelectorJSON, `"{\"node-role.kubernetes.io/worker\":\"\"}"`; got != want {
		t.Fatalf("node selector raw = %q, want %q", got, want)
	}
	if _, err := parseMatchLabelsJSON(config.NodeCacheNodeSelectorJSON); err != nil {
		t.Fatalf("parseMatchLabelsJSON(node selector) error = %v", err)
	}
	if _, err := parseMatchLabelsJSON(config.NodeCacheBlockDeviceJSON); err != nil {
		t.Fatalf("parseMatchLabelsJSON(block device selector) error = %v", err)
	}
}
