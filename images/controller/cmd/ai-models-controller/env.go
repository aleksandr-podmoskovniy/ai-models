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

const (
	logFormatEnv                          = "LOG_FORMAT"
	logLevelEnv                           = "LOG_LEVEL"
	cleanupNamespaceEnv                   = "CLEANUP_NAMESPACE"
	publicationWorkerImageEnv             = "PUBLICATION_WORKER_IMAGE"
	publicationWorkerImagePullSecretEnv   = "PUBLICATION_WORKER_IMAGE_PULL_SECRET_NAME"
	publicationWorkerNamespaceEnv         = "PUBLICATION_WORKER_NAMESPACE"
	publicationWorkerServiceAccountEnv    = "PUBLICATION_WORKER_SERVICE_ACCOUNT"
	publicationOCIRepositoryPrefixEnv     = "PUBLICATION_OCI_REPOSITORY_PREFIX"
	publicationOCIInsecureEnv             = "PUBLICATION_OCI_INSECURE"
	publicationOCISecretEnv               = "PUBLICATION_OCI_CREDENTIALS_SECRET_NAME"
	publicationOCICASecretEnv             = "PUBLICATION_OCI_CA_SECRET_NAME"
	publicationOCIDirectUploadEndpointEnv = "PUBLICATION_OCI_DIRECT_UPLOAD_ENDPOINT"
	publicationSourceFetchModeEnv         = "PUBLICATION_SOURCE_FETCH_MODE"
	publicationMaxConcurrentWorkersEnv    = "PUBLICATION_MAX_CONCURRENT_WORKERS"
	publicationWorkerCPURequestEnv        = "PUBLICATION_WORKER_CPU_REQUEST"
	publicationWorkerCPULimitEnv          = "PUBLICATION_WORKER_CPU_LIMIT"
	publicationWorkerMemoryRequestEnv     = "PUBLICATION_WORKER_MEMORY_REQUEST"
	publicationWorkerMemoryLimitEnv       = "PUBLICATION_WORKER_MEMORY_LIMIT"
	publicationWorkerEphemeralReqEnv      = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_REQUEST"
	publicationWorkerEphemeralLimitEnv    = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_LIMIT"
	artifactsBucketEnv                    = "ARTIFACTS_BUCKET"
	artifactsS3EndpointEnv                = "ARTIFACTS_S3_ENDPOINT_URL"
	artifactsS3RegionEnv                  = "ARTIFACTS_S3_REGION"
	artifactsS3UsePathStyleEnv            = "ARTIFACTS_S3_USE_PATH_STYLE"
	artifactsS3IgnoreTLSEnv               = "ARTIFACTS_S3_IGNORE_TLS"
	artifactsCredentialsSecretEnv         = "ARTIFACTS_CREDENTIALS_SECRET_NAME"
	artifactsCASecretEnv                  = "ARTIFACTS_CA_SECRET_NAME"
	artifactsCapacityLimitEnv             = "ARTIFACTS_CAPACITY_LIMIT"
	nodeCacheEnabledEnv                   = "NODE_CACHE_ENABLED"
	nodeCacheRuntimeImageEnv              = "NODE_CACHE_RUNTIME_IMAGE"
	nodeCacheCSIRegistrarImageEnv         = "NODE_CACHE_CSI_REGISTRAR_IMAGE"
	nodeCacheMaxSizeEnv                   = "NODE_CACHE_MAX_SIZE"
	nodeCacheSharedVolumeSizeEnv          = "NODE_CACHE_SHARED_VOLUME_SIZE"
	nodeCacheStorageClassNameEnv          = "NODE_CACHE_STORAGE_CLASS_NAME"
	nodeCacheVolumeGroupSetNameEnv        = "NODE_CACHE_VOLUME_GROUP_SET_NAME"
	nodeCacheVGNameOnNodeEnv              = "NODE_CACHE_VOLUME_GROUP_NAME_ON_NODE"
	nodeCacheThinPoolNameEnv              = "NODE_CACHE_THIN_POOL_NAME"
	nodeCacheNodeSelectorEnv              = "NODE_CACHE_NODE_SELECTOR_JSON"
	nodeCacheBlockDeviceSelectorEnv       = "NODE_CACHE_BLOCK_DEVICE_SELECTOR_JSON"
	uploadServiceNameEnv                  = "UPLOAD_SERVICE_NAME"
	uploadPublicHostEnv                   = "UPLOAD_PUBLIC_HOST"
	metricsBindAddressEnv                 = "METRICS_BIND_ADDRESS"
	healthProbeBindAddressEnv             = "HEALTH_PROBE_BIND_ADDRESS"
	leaderElectEnv                        = "LEADER_ELECT"
	leaderElectionIDEnv                   = "LEADER_ELECTION_ID"
	leaderElectionNamespaceEnv            = "LEADER_ELECTION_NAMESPACE"
)

const (
	defaultPublicationMaxConcurrentWorkers = 4
	defaultPublicationWorkerCPURequest     = "1"
	defaultPublicationWorkerCPULimit       = "4"
	defaultPublicationWorkerMemoryRequest  = "1Gi"
	defaultPublicationWorkerMemoryLimit    = "2Gi"
	defaultPublicationWorkerEphemeralReq   = "1Gi"
	defaultPublicationWorkerEphemeralLimit = "1Gi"
)
