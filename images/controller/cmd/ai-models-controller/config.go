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
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	defaultDMCRReadAuthSecretName   = "ai-models-dmcr-auth-read"
	defaultUploadGatewayServiceName = "ai-models-upload-gateway"
)

type managerConfig struct {
	LogFormat string
	LogLevel  string

	CleanupNamespace string

	PublicationWorkerImage                     string
	PublicationWorkerImagePullSecretName       string
	WorkloadDeliveryRuntimeImagePullSecretName string
	PublicationWorkerNamespace                 string
	PublicationWorkerServiceAccount            string
	PublicationOCIRepositoryPrefix             string
	PublicationOCIInsecure                     bool
	PublicationOCISecretName                   string
	PublicationOCICASecretName                 string
	PublicationOCIDirectUploadEndpoint         string
	PublicationSourceFetchMode                 publicationports.SourceFetchMode
	PublicationMaxConcurrentWorkers            int
	PublicationWorkerCPURequest                string
	PublicationWorkerCPULimit                  string
	PublicationWorkerMemoryRequest             string
	PublicationWorkerMemoryLimit               string
	PublicationWorkerEphemeralRequest          string
	PublicationWorkerEphemeralLimit            string

	ArtifactsBucket                string
	ArtifactsS3Endpoint            string
	ArtifactsS3Region              string
	ArtifactsS3UsePathStyle        bool
	ArtifactsS3IgnoreTLS           bool
	ArtifactsCredentialsSecretName string
	ArtifactsCASecretName          string
	ArtifactsCapacityLimit         string

	NodeCacheEnabled               bool
	NodeCacheRuntimeImage          string
	NodeCacheCSIRegistrarImage     string
	NodeCacheMaxSize               string
	NodeCacheSharedVolumeSize      string
	NodeCacheStorageClassName      string
	NodeCacheVolumeGroupSetName    string
	NodeCacheVolumeGroupNameOnNode string
	NodeCacheThinPoolName          string
	NodeCacheNodeSelectorJSON      string
	NodeCacheBlockDeviceJSON       string

	UploadServiceName string
	UploadPublicHost  string

	MetricsBindAddress      string
	HealthProbeBindAddress  string
	LeaderElect             bool
	LeaderElectionID        string
	LeaderElectionNamespace string
}

func defaultManagerConfig() managerConfig {
	return managerConfig{
		LogFormat:                            cmdsupport.EnvOr(logFormatEnv, cmdsupport.DefaultLogFormat),
		LogLevel:                             cmdsupport.EnvOr(logLevelEnv, cmdsupport.DefaultLogLevel),
		CleanupNamespace:                     cmdsupport.EnvOr(cleanupNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")),
		PublicationWorkerImage:               cmdsupport.EnvOr(publicationWorkerImageEnv, ""),
		PublicationWorkerImagePullSecretName: cmdsupport.EnvOr(publicationWorkerImagePullSecretEnv, ""),
		WorkloadDeliveryRuntimeImagePullSecretName: cmdsupport.EnvOr(
			workloadDeliveryRuntimeImagePullSecretEnv,
			cmdsupport.EnvOr(publicationWorkerImagePullSecretEnv, ""),
		),
		PublicationWorkerNamespace:         cmdsupport.EnvOr(publicationWorkerNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")),
		PublicationWorkerServiceAccount:    cmdsupport.EnvOr(publicationWorkerServiceAccountEnv, ""),
		PublicationOCIRepositoryPrefix:     cmdsupport.EnvOr(publicationOCIRepositoryPrefixEnv, ""),
		PublicationOCIInsecure:             cmdsupport.EnvOrBool(publicationOCIInsecureEnv, false),
		PublicationOCISecretName:           cmdsupport.EnvOr(publicationOCISecretEnv, ""),
		PublicationOCICASecretName:         cmdsupport.EnvOr(publicationOCICASecretEnv, ""),
		PublicationOCIDirectUploadEndpoint: cmdsupport.EnvOr(publicationOCIDirectUploadEndpointEnv, ""),
		PublicationSourceFetchMode:         publicationports.NormalizeSourceFetchMode(publicationports.SourceFetchMode(cmdsupport.EnvOr(publicationSourceFetchModeEnv, ""))),
		PublicationMaxConcurrentWorkers:    cmdsupport.EnvOrInt(publicationMaxConcurrentWorkersEnv, defaultPublicationMaxConcurrentWorkers),
		PublicationWorkerCPURequest:        cmdsupport.EnvOr(publicationWorkerCPURequestEnv, defaultPublicationWorkerCPURequest),
		PublicationWorkerCPULimit:          cmdsupport.EnvOr(publicationWorkerCPULimitEnv, defaultPublicationWorkerCPULimit),
		PublicationWorkerMemoryRequest:     cmdsupport.EnvOr(publicationWorkerMemoryRequestEnv, defaultPublicationWorkerMemoryRequest),
		PublicationWorkerMemoryLimit:       cmdsupport.EnvOr(publicationWorkerMemoryLimitEnv, defaultPublicationWorkerMemoryLimit),
		PublicationWorkerEphemeralRequest:  cmdsupport.EnvOr(publicationWorkerEphemeralReqEnv, defaultPublicationWorkerEphemeralReq),
		PublicationWorkerEphemeralLimit:    cmdsupport.EnvOr(publicationWorkerEphemeralLimitEnv, defaultPublicationWorkerEphemeralLimit),
		ArtifactsBucket:                    cmdsupport.EnvOr(artifactsBucketEnv, ""),
		ArtifactsS3Endpoint:                cmdsupport.EnvOr(artifactsS3EndpointEnv, ""),
		ArtifactsS3Region:                  cmdsupport.EnvOr(artifactsS3RegionEnv, ""),
		ArtifactsS3UsePathStyle:            cmdsupport.EnvOrBool(artifactsS3UsePathStyleEnv, false),
		ArtifactsS3IgnoreTLS:               cmdsupport.EnvOrBool(artifactsS3IgnoreTLSEnv, false),
		ArtifactsCredentialsSecretName:     cmdsupport.EnvOr(artifactsCredentialsSecretEnv, ""),
		ArtifactsCASecretName:              cmdsupport.EnvOr(artifactsCASecretEnv, ""),
		ArtifactsCapacityLimit:             cmdsupport.EnvOr(artifactsCapacityLimitEnv, ""),
		NodeCacheEnabled:                   cmdsupport.EnvOrBool(nodeCacheEnabledEnv, false),
		NodeCacheRuntimeImage:              cmdsupport.EnvOr(nodeCacheRuntimeImageEnv, ""),
		NodeCacheCSIRegistrarImage:         cmdsupport.EnvOr(nodeCacheCSIRegistrarImageEnv, ""),
		NodeCacheMaxSize:                   cmdsupport.EnvOr(nodeCacheMaxSizeEnv, "200Gi"),
		NodeCacheSharedVolumeSize:          cmdsupport.EnvOr(nodeCacheSharedVolumeSizeEnv, nodecache.DefaultSharedVolumeSize),
		NodeCacheStorageClassName:          cmdsupport.EnvOr(nodeCacheStorageClassNameEnv, "ai-models-node-cache"),
		NodeCacheVolumeGroupSetName:        cmdsupport.EnvOr(nodeCacheVolumeGroupSetNameEnv, "ai-models-node-cache"),
		NodeCacheVolumeGroupNameOnNode:     cmdsupport.EnvOr(nodeCacheVGNameOnNodeEnv, "ai-models-cache"),
		NodeCacheThinPoolName:              cmdsupport.EnvOr(nodeCacheThinPoolNameEnv, "model-cache"),
		NodeCacheNodeSelectorJSON:          cmdsupport.EnvOr(nodeCacheNodeSelectorEnv, "{}"),
		NodeCacheBlockDeviceJSON:           cmdsupport.EnvOr(nodeCacheBlockDeviceSelectorEnv, "{}"),
		UploadServiceName:                  cmdsupport.EnvOr(uploadServiceNameEnv, defaultUploadGatewayServiceName),
		UploadPublicHost:                   cmdsupport.EnvOr(uploadPublicHostEnv, ""),
		MetricsBindAddress:                 cmdsupport.EnvOr(metricsBindAddressEnv, ":8080"),
		HealthProbeBindAddress:             cmdsupport.EnvOr(healthProbeBindAddressEnv, ":8081"),
		LeaderElect:                        cmdsupport.EnvOrBool(leaderElectEnv, true),
		LeaderElectionID:                   cmdsupport.EnvOr(leaderElectionIDEnv, "ai-models-controller.deckhouse.io"),
		LeaderElectionNamespace:            cmdsupport.EnvOr(leaderElectionNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")),
	}
}

func parseManagerConfig(args []string) (managerConfig, int, error) {
	config := defaultManagerConfig()

	flags := cmdsupport.NewFlagSet("ai-models-controller")
	flags.StringVar(&config.LogFormat, "log-format", config.LogFormat, "Log format: text or json.")
	flags.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level: debug, info, warn, or error.")
	flags.StringVar(&config.CleanupNamespace, "cleanup-namespace", config.CleanupNamespace, "Namespace used for controller-owned cleanup state and DMCR garbage-collection requests.")
	flags.StringVar(&config.PublicationWorkerImage, "publication-worker-image", config.PublicationWorkerImage, "Runtime image used for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerImagePullSecretName, "publication-worker-image-pull-secret-name", config.PublicationWorkerImagePullSecretName, "Optional imagePullSecret name used by publication worker Pods.")
	flags.StringVar(&config.WorkloadDeliveryRuntimeImagePullSecretName, "workload-delivery-runtime-image-pull-secret-name", config.WorkloadDeliveryRuntimeImagePullSecretName, "Optional imagePullSecret name projected into managed workloads so the materialize bridge init container can pull the runtime image.")
	flags.StringVar(&config.PublicationWorkerNamespace, "publication-worker-namespace", config.PublicationWorkerNamespace, "Namespace where publication worker Pods are created.")
	flags.StringVar(&config.PublicationWorkerServiceAccount, "publication-worker-service-account", config.PublicationWorkerServiceAccount, "ServiceAccountName used by publication worker Pods.")
	flags.StringVar(&config.PublicationOCIRepositoryPrefix, "publication-oci-repository-prefix", config.PublicationOCIRepositoryPrefix, "OCI repository prefix used by publication workers.")
	flags.BoolVar(&config.PublicationOCIInsecure, "publication-oci-insecure", config.PublicationOCIInsecure, "Disable TLS verification for publication worker OCI registry access.")
	flags.StringVar(&config.PublicationOCISecretName, "publication-oci-credentials-secret-name", config.PublicationOCISecretName, "Secret with OCI registry username/password for publication workers.")
	flags.StringVar(&config.PublicationOCICASecretName, "publication-oci-ca-secret-name", config.PublicationOCICASecretName, "Optional Secret with ca.crt for publication worker OCI registry trust.")
	flags.StringVar(&config.PublicationOCIDirectUploadEndpoint, "publication-oci-direct-upload-endpoint", config.PublicationOCIDirectUploadEndpoint, "Internal DMCR direct-upload HTTPS endpoint used to stream published blob payloads into backing storage.")
	flags.StringVar((*string)(&config.PublicationSourceFetchMode), "publication-source-fetch-mode", string(config.PublicationSourceFetchMode), "Remote source fetch mode for publication workers: mirror or direct.")
	flags.IntVar(&config.PublicationMaxConcurrentWorkers, "publication-max-concurrent-workers", config.PublicationMaxConcurrentWorkers, "Maximum number of active publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerCPURequest, "publication-worker-cpu-request", config.PublicationWorkerCPURequest, "CPU request for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerCPULimit, "publication-worker-cpu-limit", config.PublicationWorkerCPULimit, "CPU limit for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerMemoryRequest, "publication-worker-memory-request", config.PublicationWorkerMemoryRequest, "Memory request for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerMemoryLimit, "publication-worker-memory-limit", config.PublicationWorkerMemoryLimit, "Memory limit for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerEphemeralRequest, "publication-worker-ephemeral-storage-request", config.PublicationWorkerEphemeralRequest, "Ephemeral-storage request for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerEphemeralLimit, "publication-worker-ephemeral-storage-limit", config.PublicationWorkerEphemeralLimit, "Ephemeral-storage limit for publication worker Pods.")
	flags.StringVar(&config.ArtifactsBucket, "artifacts-bucket", config.ArtifactsBucket, "Bucket used for upload staging.")
	flags.StringVar(&config.ArtifactsS3Endpoint, "artifacts-s3-endpoint-url", config.ArtifactsS3Endpoint, "S3-compatible endpoint used for upload staging.")
	flags.StringVar(&config.ArtifactsS3Region, "artifacts-s3-region", config.ArtifactsS3Region, "S3-compatible region used for upload staging.")
	flags.BoolVar(&config.ArtifactsS3UsePathStyle, "artifacts-s3-use-path-style", config.ArtifactsS3UsePathStyle, "Use path-style addressing for upload staging object storage.")
	flags.BoolVar(&config.ArtifactsS3IgnoreTLS, "artifacts-s3-ignore-tls", config.ArtifactsS3IgnoreTLS, "Disable TLS verification for upload staging object storage.")
	flags.StringVar(&config.ArtifactsCredentialsSecretName, "artifacts-credentials-secret-name", config.ArtifactsCredentialsSecretName, "Secret with object storage accessKey/secretKey for upload staging.")
	flags.StringVar(&config.ArtifactsCASecretName, "artifacts-ca-secret-name", config.ArtifactsCASecretName, "Optional Secret with ca.crt for upload staging object storage.")
	flags.StringVar(&config.ArtifactsCapacityLimit, "artifacts-capacity-limit", config.ArtifactsCapacityLimit, "Optional total artifact storage capacity limit.")
	flags.BoolVar(&config.NodeCacheEnabled, "node-cache-enabled", config.NodeCacheEnabled, "Enable ai-models-managed node-local cache substrate.")
	flags.StringVar(&config.NodeCacheRuntimeImage, "node-cache-runtime-image", config.NodeCacheRuntimeImage, "Internal node-cache runtime image used by the managed CSI runtime pod.")
	flags.StringVar(&config.NodeCacheCSIRegistrarImage, "node-cache-csi-registrar-image", config.NodeCacheCSIRegistrarImage, "node-driver-registrar image used by the managed node-cache CSI runtime pod.")
	flags.StringVar(&config.NodeCacheMaxSize, "node-cache-max-size", config.NodeCacheMaxSize, "Per-node thin-pool size budget for managed node-local cache substrate.")
	flags.StringVar(&config.NodeCacheSharedVolumeSize, "node-cache-shared-volume-size", config.NodeCacheSharedVolumeSize, "Stable per-node shared cache PVC size used by the managed node-cache runtime plane.")
	flags.StringVar(&config.NodeCacheStorageClassName, "node-cache-storage-class-name", config.NodeCacheStorageClassName, "Managed LocalStorageClass name for node-local cache substrate.")
	flags.StringVar(&config.NodeCacheVolumeGroupSetName, "node-cache-volume-group-set-name", config.NodeCacheVolumeGroupSetName, "Managed LVMVolumeGroupSet name for node-local cache substrate.")
	flags.StringVar(&config.NodeCacheVolumeGroupNameOnNode, "node-cache-volume-group-name-on-node", config.NodeCacheVolumeGroupNameOnNode, "Actual VG name created on nodes for node-local cache substrate.")
	flags.StringVar(&config.NodeCacheThinPoolName, "node-cache-thin-pool-name", config.NodeCacheThinPoolName, "Thin-pool name used for managed node-local cache substrate.")
	flags.StringVar(&config.NodeCacheNodeSelectorJSON, "node-cache-node-selector-json", config.NodeCacheNodeSelectorJSON, "JSON object with matchLabels for node-local cache substrate node selection.")
	flags.StringVar(&config.NodeCacheBlockDeviceJSON, "node-cache-block-device-selector-json", config.NodeCacheBlockDeviceJSON, "JSON object with matchLabels for BlockDevice selection in managed node-local cache substrate.")
	flags.StringVar(&config.UploadServiceName, "upload-service-name", config.UploadServiceName, "Upload gateway Service name used for upload session URLs.")
	flags.StringVar(&config.UploadPublicHost, "upload-public-host", config.UploadPublicHost, "Public host used for upload session ingress URLs.")
	flags.StringVar(&config.MetricsBindAddress, "metrics-bind-address", config.MetricsBindAddress, "The address the metric endpoint binds to.")
	flags.StringVar(&config.HealthProbeBindAddress, "health-probe-bind-address", config.HealthProbeBindAddress, "The address the health probe endpoint binds to.")
	flags.BoolVar(&config.LeaderElect, "leader-elect", config.LeaderElect, "Enable leader election for controller manager.")
	flags.StringVar(&config.LeaderElectionID, "leader-election-id", config.LeaderElectionID, "Leader election ID used for controller manager leases.")
	flags.StringVar(&config.LeaderElectionNamespace, "leader-election-namespace", config.LeaderElectionNamespace, "Namespace used for leader election leases.")
	if err := flags.Parse(args); err != nil {
		return managerConfig{}, 2, err
	}
	nodeSelectorLabels, err := parseMatchLabelsJSON(config.NodeCacheNodeSelectorJSON)
	if err != nil {
		return managerConfig{}, 2, err
	}
	blockDeviceSelectorLabels, err := parseMatchLabelsJSON(config.NodeCacheBlockDeviceJSON)
	if err != nil {
		return managerConfig{}, 2, err
	}
	if config.NodeCacheEnabled && len(nodeSelectorLabels) == 0 {
		return managerConfig{}, 2, fmt.Errorf("node-cache-node-selector-json must not be empty when node cache is enabled")
	}
	if config.NodeCacheEnabled && len(blockDeviceSelectorLabels) == 0 {
		return managerConfig{}, 2, fmt.Errorf("node-cache-block-device-selector-json must not be empty when node cache is enabled")
	}
	if config.NodeCacheEnabled && strings.TrimSpace(config.NodeCacheRuntimeImage) == "" {
		return managerConfig{}, 2, fmt.Errorf("node-cache-runtime-image must not be empty when node cache is enabled")
	}
	if config.NodeCacheEnabled && strings.TrimSpace(config.NodeCacheCSIRegistrarImage) == "" {
		return managerConfig{}, 2, fmt.Errorf("node-cache-csi-registrar-image must not be empty when node cache is enabled")
	}
	if config.NodeCacheEnabled {
		if err := validateNodeCacheStorageSizes(config.NodeCacheMaxSize, config.NodeCacheSharedVolumeSize); err != nil {
			return managerConfig{}, 2, err
		}
	}
	if _, err := cmdsupport.ParseOptionalPositiveBytesQuantity(config.ArtifactsCapacityLimit, "artifacts-capacity-limit"); err != nil {
		return managerConfig{}, 2, err
	}

	return config, 0, nil
}

func validateNodeCacheStorageSizes(maxSizeValue, sharedVolumeSizeValue string) error {
	maxSize, err := resource.ParseQuantity(strings.TrimSpace(maxSizeValue))
	if err != nil {
		return fmt.Errorf("node-cache-max-size must be a valid quantity")
	}
	sharedVolumeSize, err := resource.ParseQuantity(strings.TrimSpace(sharedVolumeSizeValue))
	if err != nil {
		return fmt.Errorf("node-cache-shared-volume-size must be a valid quantity")
	}
	if sharedVolumeSize.Cmp(maxSize) > 0 {
		return fmt.Errorf("node-cache-shared-volume-size must not exceed node-cache-max-size")
	}
	return nil
}
