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
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	"github.com/deckhouse/ai-models/controller/internal/bootstrap"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
)

const defaultDMCRReadAuthSecretName = "ai-models-dmcr-auth-read"

type managerConfig struct {
	LogFormat string
	LogLevel  string

	CleanupJobImage               string
	CleanupJobImagePullSecretName string
	CleanupJobNamespace           string
	CleanupJobServiceAccount      string
	CleanupJobEnvPassThrough      string

	PublicationWorkerImage               string
	PublicationWorkerImagePullSecretName string
	PublicationWorkerNamespace           string
	PublicationWorkerServiceAccount      string
	PublicationOCIRepositoryPrefix       string
	PublicationOCIInsecure               bool
	PublicationOCISecretName             string
	PublicationOCICASecretName           string
	PublicationOCIDirectUploadEndpoint   string
	PublicationSourceAcquisitionMode     publicationports.SourceAcquisitionMode
	PublicationMaxConcurrentWorkers      int
	PublicationWorkerCPURequest          string
	PublicationWorkerCPULimit            string
	PublicationWorkerMemoryRequest       string
	PublicationWorkerMemoryLimit         string
	PublicationWorkerEphemeralRequest    string
	PublicationWorkerEphemeralLimit      string

	ArtifactsBucket                string
	ArtifactsS3Endpoint            string
	ArtifactsS3Region              string
	ArtifactsS3UsePathStyle        bool
	ArtifactsS3IgnoreTLS           bool
	ArtifactsCredentialsSecretName string
	ArtifactsCASecretName          string

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
		CleanupJobImage:                      cmdsupport.EnvOr(cleanupJobImageEnv, ""),
		CleanupJobImagePullSecretName:        cmdsupport.EnvOr(cleanupJobImagePullSecretEnv, ""),
		CleanupJobNamespace:                  cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")),
		CleanupJobServiceAccount:             cmdsupport.EnvOr(cleanupJobServiceAccountEnv, ""),
		CleanupJobEnvPassThrough:             cmdsupport.EnvOr(cleanupJobEnvPassThroughEnv, defaultCleanupPassThrough),
		PublicationWorkerImage:               cmdsupport.EnvOr(publicationWorkerImageEnv, cmdsupport.EnvOr(cleanupJobImageEnv, "")),
		PublicationWorkerImagePullSecretName: cmdsupport.EnvOr(publicationWorkerImagePullSecretEnv, cmdsupport.EnvOr(cleanupJobImagePullSecretEnv, "")),
		PublicationWorkerNamespace:           cmdsupport.EnvOr(publicationWorkerNamespaceEnv, cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models"))),
		PublicationWorkerServiceAccount:      cmdsupport.EnvOr(publicationWorkerServiceAccountEnv, cmdsupport.EnvOr(cleanupJobServiceAccountEnv, "")),
		PublicationOCIRepositoryPrefix:       cmdsupport.EnvOr(publicationOCIRepositoryPrefixEnv, ""),
		PublicationOCIInsecure:               cmdsupport.EnvOrBool(publicationOCIInsecureEnv, false),
		PublicationOCISecretName:             cmdsupport.EnvOr(publicationOCISecretEnv, ""),
		PublicationOCICASecretName:           cmdsupport.EnvOr(publicationOCICASecretEnv, ""),
		PublicationOCIDirectUploadEndpoint:   cmdsupport.EnvOr(publicationOCIDirectUploadEndpointEnv, ""),
		PublicationSourceAcquisitionMode:     publicationports.NormalizeSourceAcquisitionMode(publicationports.SourceAcquisitionMode(cmdsupport.EnvOr(publicationSourceAcquisitionModeEnv, ""))),
		PublicationMaxConcurrentWorkers:      cmdsupport.EnvOrInt(publicationMaxConcurrentWorkersEnv, defaultPublicationMaxConcurrentWorkers),
		PublicationWorkerCPURequest:          cmdsupport.EnvOr(publicationWorkerCPURequestEnv, defaultPublicationWorkerCPURequest),
		PublicationWorkerCPULimit:            cmdsupport.EnvOr(publicationWorkerCPULimitEnv, defaultPublicationWorkerCPULimit),
		PublicationWorkerMemoryRequest:       cmdsupport.EnvOr(publicationWorkerMemoryRequestEnv, defaultPublicationWorkerMemoryRequest),
		PublicationWorkerMemoryLimit:         cmdsupport.EnvOr(publicationWorkerMemoryLimitEnv, defaultPublicationWorkerMemoryLimit),
		PublicationWorkerEphemeralRequest:    cmdsupport.EnvOr(publicationWorkerEphemeralReqEnv, defaultPublicationWorkerEphemeralReq),
		PublicationWorkerEphemeralLimit:      cmdsupport.EnvOr(publicationWorkerEphemeralLimitEnv, defaultPublicationWorkerEphemeralLimit),
		ArtifactsBucket:                      cmdsupport.EnvOr(artifactsBucketEnv, ""),
		ArtifactsS3Endpoint:                  cmdsupport.EnvOr(artifactsS3EndpointEnv, ""),
		ArtifactsS3Region:                    cmdsupport.EnvOr(artifactsS3RegionEnv, ""),
		ArtifactsS3UsePathStyle:              cmdsupport.EnvOrBool(artifactsS3UsePathStyleEnv, false),
		ArtifactsS3IgnoreTLS:                 cmdsupport.EnvOrBool(artifactsS3IgnoreTLSEnv, false),
		ArtifactsCredentialsSecretName:       cmdsupport.EnvOr(artifactsCredentialsSecretEnv, ""),
		ArtifactsCASecretName:                cmdsupport.EnvOr(artifactsCASecretEnv, ""),
		UploadServiceName:                    cmdsupport.EnvOr(uploadServiceNameEnv, "ai-models-controller"),
		UploadPublicHost:                     cmdsupport.EnvOr(uploadPublicHostEnv, ""),
		MetricsBindAddress:                   cmdsupport.EnvOr(metricsBindAddressEnv, ":8080"),
		HealthProbeBindAddress:               cmdsupport.EnvOr(healthProbeBindAddressEnv, ":8081"),
		LeaderElect:                          cmdsupport.EnvOrBool(leaderElectEnv, true),
		LeaderElectionID:                     cmdsupport.EnvOr(leaderElectionIDEnv, "ai-models-controller.deckhouse.io"),
		LeaderElectionNamespace:              cmdsupport.EnvOr(leaderElectionNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")),
	}
}

func parseManagerConfig(args []string) (managerConfig, int, error) {
	config := defaultManagerConfig()

	flags := cmdsupport.NewFlagSet("ai-models-controller")
	flags.StringVar(&config.LogFormat, "log-format", config.LogFormat, "Log format: text or json.")
	flags.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level: debug, info, warn, or error.")
	flags.StringVar(&config.CleanupJobImage, "cleanup-job-image", config.CleanupJobImage, "Runtime image used for cleanup Jobs.")
	flags.StringVar(&config.CleanupJobImagePullSecretName, "cleanup-job-image-pull-secret-name", config.CleanupJobImagePullSecretName, "Optional imagePullSecret name used by cleanup Jobs.")
	flags.StringVar(&config.CleanupJobNamespace, "cleanup-job-namespace", config.CleanupJobNamespace, "Namespace where cleanup Jobs are created.")
	flags.StringVar(&config.CleanupJobServiceAccount, "cleanup-job-service-account", config.CleanupJobServiceAccount, "ServiceAccountName used by cleanup Jobs.")
	flags.StringVar(&config.CleanupJobEnvPassThrough, "cleanup-job-env-pass-through", config.CleanupJobEnvPassThrough, "Comma-separated list of controller env vars copied into cleanup Jobs.")
	flags.StringVar(&config.PublicationWorkerImage, "publication-worker-image", config.PublicationWorkerImage, "Runtime image used for publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerImagePullSecretName, "publication-worker-image-pull-secret-name", config.PublicationWorkerImagePullSecretName, "Optional imagePullSecret name used by publication worker Pods.")
	flags.StringVar(&config.PublicationWorkerNamespace, "publication-worker-namespace", config.PublicationWorkerNamespace, "Namespace where publication worker Pods are created.")
	flags.StringVar(&config.PublicationWorkerServiceAccount, "publication-worker-service-account", config.PublicationWorkerServiceAccount, "ServiceAccountName used by publication worker Pods.")
	flags.StringVar(&config.PublicationOCIRepositoryPrefix, "publication-oci-repository-prefix", config.PublicationOCIRepositoryPrefix, "OCI repository prefix used by publication workers.")
	flags.BoolVar(&config.PublicationOCIInsecure, "publication-oci-insecure", config.PublicationOCIInsecure, "Disable TLS verification for publication worker OCI registry access.")
	flags.StringVar(&config.PublicationOCISecretName, "publication-oci-credentials-secret-name", config.PublicationOCISecretName, "Secret with OCI registry username/password for publication workers.")
	flags.StringVar(&config.PublicationOCICASecretName, "publication-oci-ca-secret-name", config.PublicationOCICASecretName, "Optional Secret with ca.crt for publication worker OCI registry trust.")
	flags.StringVar(&config.PublicationOCIDirectUploadEndpoint, "publication-oci-direct-upload-endpoint", config.PublicationOCIDirectUploadEndpoint, "Internal DMCR direct-upload HTTPS endpoint used for heavy layer blob uploads.")
	flags.StringVar((*string)(&config.PublicationSourceAcquisitionMode), "publication-source-acquisition-mode", string(config.PublicationSourceAcquisitionMode), "Source acquisition mode for publication workers: mirror or direct.")
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
	flags.StringVar(&config.UploadServiceName, "upload-service-name", config.UploadServiceName, "Shared Service name used for upload gateway URLs.")
	flags.StringVar(&config.UploadPublicHost, "upload-public-host", config.UploadPublicHost, "Public host used for upload session ingress URLs.")
	flags.StringVar(&config.MetricsBindAddress, "metrics-bind-address", config.MetricsBindAddress, "The address the metric endpoint binds to.")
	flags.StringVar(&config.HealthProbeBindAddress, "health-probe-bind-address", config.HealthProbeBindAddress, "The address the health probe endpoint binds to.")
	flags.BoolVar(&config.LeaderElect, "leader-elect", config.LeaderElect, "Enable leader election for controller manager.")
	flags.StringVar(&config.LeaderElectionID, "leader-election-id", config.LeaderElectionID, "Leader election ID used for controller manager leases.")
	flags.StringVar(&config.LeaderElectionNamespace, "leader-election-namespace", config.LeaderElectionNamespace, "Namespace used for leader election leases.")
	if err := flags.Parse(args); err != nil {
		return managerConfig{}, 2, err
	}

	return config, 0, nil
}

func (c managerConfig) objectStorageOptions() storageprojection.Options {
	return storageprojection.Options{
		Bucket:                c.ArtifactsBucket,
		EndpointURL:           c.ArtifactsS3Endpoint,
		Region:                c.ArtifactsS3Region,
		UsePathStyle:          c.ArtifactsS3UsePathStyle,
		Insecure:              c.ArtifactsS3IgnoreTLS,
		CredentialsSecretName: c.ArtifactsCredentialsSecretName,
		CASecretName:          c.ArtifactsCASecretName,
	}
}

func (c managerConfig) bootstrapOptions(resources corev1.ResourceRequirements) bootstrap.Options {
	artifactsObjectStorage := c.objectStorageOptions()

	return bootstrap.Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:               c.CleanupJobNamespace,
				Image:                   c.CleanupJobImage,
				ImagePullSecretName:     c.CleanupJobImagePullSecretName,
				ServiceAccountName:      c.CleanupJobServiceAccount,
				OCIInsecure:             c.PublicationOCIInsecure,
				OCIRegistrySecretName:   c.PublicationOCISecretName,
				OCIRegistryCASecretName: c.PublicationOCICASecretName,
				ObjectStorage:           artifactsObjectStorage,
				Env:                     cleanupJobEnv(c.CleanupJobEnvPassThrough, c.LogFormat, c.LogLevel),
			},
			RequeueAfter: 5 * time.Second,
		},
		PublicationRuntime: catalogstatus.Options{
			RuntimeLogFormat: c.LogFormat,
			RuntimeLogLevel:  c.LogLevel,
			Runtime: sourceworker.RuntimeOptions{
				Namespace:               c.PublicationWorkerNamespace,
				Image:                   cmdsupport.FallbackString(c.PublicationWorkerImage, c.CleanupJobImage),
				ImagePullSecretName:     cmdsupport.FallbackString(c.PublicationWorkerImagePullSecretName, c.CleanupJobImagePullSecretName),
				ServiceAccountName:      cmdsupport.FallbackString(c.PublicationWorkerServiceAccount, c.CleanupJobServiceAccount),
				OCIRepositoryPrefix:     c.PublicationOCIRepositoryPrefix,
				OCIInsecure:             c.PublicationOCIInsecure,
				OCIRegistrySecretName:   c.PublicationOCISecretName,
				OCIRegistryCASecretName: c.PublicationOCICASecretName,
				OCIDirectUploadEndpoint: c.PublicationOCIDirectUploadEndpoint,
				ObjectStorage:           artifactsObjectStorage,
				SourceAcquisition:       c.PublicationSourceAcquisitionMode,
				Resources:               resources,
			},
			MaxConcurrentWorkers: c.PublicationMaxConcurrentWorkers,
			UploadGateway: catalogstatus.UploadGatewayOptions{
				ServiceName: c.UploadServiceName,
				PublicHost:  c.UploadPublicHost,
			},
		},
		WorkloadDelivery: workloaddelivery.Options{
			Service: modeldelivery.ServiceOptions{
				Render: modeldelivery.Options{
					RuntimeImage:   cmdsupport.FallbackString(c.PublicationWorkerImage, c.CleanupJobImage),
					LogFormat:      c.LogFormat,
					LogLevel:       c.LogLevel,
					OCIInsecure:    c.PublicationOCIInsecure,
					CacheMountPath: modeldelivery.DefaultCacheMountPath,
				},
				RegistrySourceNamespace:      cmdsupport.FallbackString(c.PublicationWorkerNamespace, c.CleanupJobNamespace),
				RegistrySourceAuthSecretName: defaultDMCRReadAuthSecretName,
				RegistrySourceCASecretName:   c.PublicationOCICASecretName,
			},
		},
		Runtime: bootstrap.RuntimeOptions{
			MetricsBindAddress:      c.MetricsBindAddress,
			HealthProbeBindAddress:  c.HealthProbeBindAddress,
			LeaderElection:          c.LeaderElect,
			LeaderElectionID:        c.LeaderElectionID,
			LeaderElectionNamespace: c.LeaderElectionNamespace,
		},
	}
}
