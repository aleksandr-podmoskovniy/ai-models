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

	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	corev1 "k8s.io/api/core/v1"
)

const (
	logFormatEnv                              = "LOG_FORMAT"
	logLevelEnv                               = "LOG_LEVEL"
	cleanupJobImageEnv                        = "CLEANUP_JOB_IMAGE"
	cleanupJobImagePullSecretEnv              = "CLEANUP_JOB_IMAGE_PULL_SECRET_NAME"
	cleanupJobNamespaceEnv                    = "CLEANUP_JOB_NAMESPACE"
	cleanupJobServiceAccountEnv               = "CLEANUP_JOB_SERVICE_ACCOUNT"
	cleanupJobEnvPassThroughEnv               = "CLEANUP_JOB_ENV_PASS_THROUGH"
	publicationWorkerImageEnv                 = "PUBLICATION_WORKER_IMAGE"
	publicationWorkerImagePullSecretEnv       = "PUBLICATION_WORKER_IMAGE_PULL_SECRET_NAME"
	workloadDeliveryRuntimeImagePullSecretEnv = "WORKLOAD_DELIVERY_RUNTIME_IMAGE_PULL_SECRET_NAME"
	publicationWorkerNamespaceEnv             = "PUBLICATION_WORKER_NAMESPACE"
	publicationWorkerServiceAccountEnv        = "PUBLICATION_WORKER_SERVICE_ACCOUNT"
	publicationOCIRepositoryPrefixEnv         = "PUBLICATION_OCI_REPOSITORY_PREFIX"
	publicationOCIInsecureEnv                 = "PUBLICATION_OCI_INSECURE"
	publicationOCISecretEnv                   = "PUBLICATION_OCI_CREDENTIALS_SECRET_NAME"
	publicationOCICASecretEnv                 = "PUBLICATION_OCI_CA_SECRET_NAME"
	publicationOCIDirectUploadEndpointEnv     = "PUBLICATION_OCI_DIRECT_UPLOAD_ENDPOINT"
	publicationSourceFetchModeEnv             = "PUBLICATION_SOURCE_FETCH_MODE"
	publicationMaxConcurrentWorkersEnv        = "PUBLICATION_MAX_CONCURRENT_WORKERS"
	publicationWorkerCPURequestEnv            = "PUBLICATION_WORKER_CPU_REQUEST"
	publicationWorkerCPULimitEnv              = "PUBLICATION_WORKER_CPU_LIMIT"
	publicationWorkerMemoryRequestEnv         = "PUBLICATION_WORKER_MEMORY_REQUEST"
	publicationWorkerMemoryLimitEnv           = "PUBLICATION_WORKER_MEMORY_LIMIT"
	publicationWorkerEphemeralReqEnv          = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_REQUEST"
	publicationWorkerEphemeralLimitEnv        = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_LIMIT"
	artifactsBucketEnv                        = "ARTIFACTS_BUCKET"
	artifactsS3EndpointEnv                    = "ARTIFACTS_S3_ENDPOINT_URL"
	artifactsS3RegionEnv                      = "ARTIFACTS_S3_REGION"
	artifactsS3UsePathStyleEnv                = "ARTIFACTS_S3_USE_PATH_STYLE"
	artifactsS3IgnoreTLSEnv                   = "ARTIFACTS_S3_IGNORE_TLS"
	artifactsCredentialsSecretEnv             = "ARTIFACTS_CREDENTIALS_SECRET_NAME"
	artifactsCASecretEnv                      = "ARTIFACTS_CA_SECRET_NAME"
	nodeCacheEnabledEnv                       = "NODE_CACHE_ENABLED"
	nodeCacheMaxSizeEnv                       = "NODE_CACHE_MAX_SIZE"
	nodeCacheSharedVolumeSizeEnv              = "NODE_CACHE_SHARED_VOLUME_SIZE"
	nodeCacheFallbackVolumeSizeEnv            = "NODE_CACHE_FALLBACK_VOLUME_SIZE"
	nodeCacheStorageClassNameEnv              = "NODE_CACHE_STORAGE_CLASS_NAME"
	nodeCacheVolumeGroupSetNameEnv            = "NODE_CACHE_VOLUME_GROUP_SET_NAME"
	nodeCacheVGNameOnNodeEnv                  = "NODE_CACHE_VOLUME_GROUP_NAME_ON_NODE"
	nodeCacheThinPoolNameEnv                  = "NODE_CACHE_THIN_POOL_NAME"
	nodeCacheNodeSelectorEnv                  = "NODE_CACHE_NODE_SELECTOR_JSON"
	nodeCacheBlockDeviceSelectorEnv           = "NODE_CACHE_BLOCK_DEVICE_SELECTOR_JSON"
	uploadServiceNameEnv                      = "UPLOAD_SERVICE_NAME"
	uploadPublicHostEnv                       = "UPLOAD_PUBLIC_HOST"
	metricsBindAddressEnv                     = "METRICS_BIND_ADDRESS"
	healthProbeBindAddressEnv                 = "HEALTH_PROBE_BIND_ADDRESS"
	leaderElectEnv                            = "LEADER_ELECT"
	leaderElectionIDEnv                       = "LEADER_ELECTION_ID"
	leaderElectionNamespaceEnv                = "LEADER_ELECTION_NAMESPACE"
)

const defaultCleanupPassThrough = "LOG_FORMAT,LOG_LEVEL,SSL_CERT_FILE,REQUESTS_CA_BUNDLE,AWS_CA_BUNDLE"

const (
	defaultPublicationMaxConcurrentWorkers = 1
	defaultPublicationWorkerCPURequest     = "1"
	defaultPublicationWorkerCPULimit       = "4"
	defaultPublicationWorkerMemoryRequest  = "8Gi"
	defaultPublicationWorkerMemoryLimit    = "16Gi"
	defaultPublicationWorkerEphemeralReq   = "1Gi"
	defaultPublicationWorkerEphemeralLimit = "1Gi"
)

func cleanupJobEnv(passThrough, logFormat, logLevel string) []corev1.EnvVar {
	env := cmdsupport.PassThroughEnv(passThrough)
	env = upsertEnvValue(env, logFormatEnv, logFormat)
	env = upsertEnvValue(env, logLevelEnv, logLevel)
	return env
}

func upsertEnvValue(env []corev1.EnvVar, name, value string) []corev1.EnvVar {
	if strings.TrimSpace(value) == "" {
		return env
	}
	for index := range env {
		if env[index].Name == name {
			env[index].Value = value
			return env
		}
	}
	return append(env, corev1.EnvVar{Name: name, Value: value})
}
