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
	"log/slog"
	"os"
	"time"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	"github.com/deckhouse/ai-models/controller/internal/bootstrap"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	logFormatEnv                        = "LOG_FORMAT"
	cleanupJobImageEnv                  = "CLEANUP_JOB_IMAGE"
	cleanupJobImagePullSecretEnv        = "CLEANUP_JOB_IMAGE_PULL_SECRET_NAME"
	cleanupJobNamespaceEnv              = "CLEANUP_JOB_NAMESPACE"
	cleanupJobServiceAccountEnv         = "CLEANUP_JOB_SERVICE_ACCOUNT"
	cleanupJobEnvPassThroughEnv         = "CLEANUP_JOB_ENV_PASS_THROUGH"
	publicationWorkerImageEnv           = "PUBLICATION_WORKER_IMAGE"
	publicationWorkerImagePullSecretEnv = "PUBLICATION_WORKER_IMAGE_PULL_SECRET_NAME"
	publicationWorkerNamespaceEnv       = "PUBLICATION_WORKER_NAMESPACE"
	publicationWorkerServiceAccountEnv  = "PUBLICATION_WORKER_SERVICE_ACCOUNT"
	publicationOCIRepositoryPrefixEnv   = "PUBLICATION_OCI_REPOSITORY_PREFIX"
	publicationOCIInsecureEnv           = "PUBLICATION_OCI_INSECURE"
	publicationOCISecretEnv             = "PUBLICATION_OCI_CREDENTIALS_SECRET_NAME"
	publicationOCICASecretEnv           = "PUBLICATION_OCI_CA_SECRET_NAME"
	publicationMaxConcurrentWorkersEnv  = "PUBLICATION_MAX_CONCURRENT_WORKERS"
	publicationWorkVolumeTypeEnv        = "PUBLICATION_WORK_VOLUME_TYPE"
	publicationWorkVolumeSizeLimitEnv   = "PUBLICATION_WORK_VOLUME_SIZE_LIMIT"
	publicationWorkVolumeClaimNameEnv   = "PUBLICATION_WORK_VOLUME_CLAIM_NAME"
	publicationWorkerCPURequestEnv      = "PUBLICATION_WORKER_CPU_REQUEST"
	publicationWorkerCPULimitEnv        = "PUBLICATION_WORKER_CPU_LIMIT"
	publicationWorkerMemoryRequestEnv   = "PUBLICATION_WORKER_MEMORY_REQUEST"
	publicationWorkerMemoryLimitEnv     = "PUBLICATION_WORKER_MEMORY_LIMIT"
	publicationWorkerEphemeralReqEnv    = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_REQUEST"
	publicationWorkerEphemeralLimitEnv  = "PUBLICATION_WORKER_EPHEMERAL_STORAGE_LIMIT"
	artifactsBucketEnv                  = "ARTIFACTS_BUCKET"
	artifactsS3EndpointEnv              = "ARTIFACTS_S3_ENDPOINT_URL"
	artifactsS3RegionEnv                = "ARTIFACTS_S3_REGION"
	artifactsS3UsePathStyleEnv          = "ARTIFACTS_S3_USE_PATH_STYLE"
	artifactsS3IgnoreTLSEnv             = "ARTIFACTS_S3_IGNORE_TLS"
	artifactsCredentialsSecretEnv       = "ARTIFACTS_CREDENTIALS_SECRET_NAME"
	artifactsCASecretEnv                = "ARTIFACTS_CA_SECRET_NAME"
	uploadServiceNameEnv                = "UPLOAD_SERVICE_NAME"
	uploadPublicHostEnv                 = "UPLOAD_PUBLIC_HOST"
	metricsBindAddressEnv               = "METRICS_BIND_ADDRESS"
	healthProbeBindAddressEnv           = "HEALTH_PROBE_BIND_ADDRESS"
	leaderElectEnv                      = "LEADER_ELECT"
	leaderElectionIDEnv                 = "LEADER_ELECTION_ID"
	leaderElectionNamespaceEnv          = "LEADER_ELECTION_NAMESPACE"
)

const defaultCleanupPassThrough = "SSL_CERT_FILE,REQUESTS_CA_BUNDLE,AWS_CA_BUNDLE"

const (
	defaultPublicationMaxConcurrentWorkers = 1
	defaultPublicationWorkVolumeSizeLimit  = "50Gi"
	defaultPublicationWorkVolumeClaimName  = "ai-models-publication-work"
	defaultPublicationWorkerCPURequest     = "1"
	defaultPublicationWorkerCPULimit       = "4"
	defaultPublicationWorkerMemoryRequest  = "8Gi"
	defaultPublicationWorkerMemoryLimit    = "16Gi"
	defaultPublicationWorkerEphemeralLimit = "50Gi"
)

func runManager(args []string) int {
	var logFormat string
	var cleanupJobImage string
	var cleanupJobImagePullSecretName string
	var cleanupJobNamespace string
	var cleanupJobServiceAccount string
	var cleanupJobEnvPassThrough string
	var publicationWorkerImage string
	var publicationWorkerImagePullSecretName string
	var publicationWorkerNamespace string
	var publicationWorkerServiceAccount string
	var publicationOCIRepositoryPrefix string
	var publicationOCIInsecure bool
	var publicationOCISecretName string
	var publicationOCICASecretName string
	var publicationMaxConcurrentWorkers int
	var publicationWorkVolumeType string
	var publicationWorkVolumeSizeLimit string
	var publicationWorkVolumeClaimName string
	var publicationWorkerCPURequest string
	var publicationWorkerCPULimit string
	var publicationWorkerMemoryRequest string
	var publicationWorkerMemoryLimit string
	var publicationWorkerEphemeralRequest string
	var publicationWorkerEphemeralLimit string
	var artifactsBucket string
	var artifactsS3Endpoint string
	var artifactsS3Region string
	var artifactsS3UsePathStyle bool
	var artifactsS3IgnoreTLS bool
	var artifactsCredentialsSecretName string
	var artifactsCASecretName string
	var uploadServiceName string
	var uploadPublicHost string
	var metricsBindAddress string
	var healthProbeBindAddress string
	var leaderElect bool
	var leaderElectionID string
	var leaderElectionNamespace string

	flags := cmdsupport.NewFlagSet("ai-models-controller")
	flags.StringVar(&logFormat, "log-format", cmdsupport.EnvOr(logFormatEnv, "text"), "Log format: text or json.")
	flags.StringVar(&cleanupJobImage, "cleanup-job-image", cmdsupport.EnvOr(cleanupJobImageEnv, ""), "Runtime image used for cleanup Jobs.")
	flags.StringVar(&cleanupJobImagePullSecretName, "cleanup-job-image-pull-secret-name", cmdsupport.EnvOr(cleanupJobImagePullSecretEnv, ""), "Optional imagePullSecret name used by cleanup Jobs.")
	flags.StringVar(&cleanupJobNamespace, "cleanup-job-namespace", cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")), "Namespace where cleanup Jobs are created.")
	flags.StringVar(&cleanupJobServiceAccount, "cleanup-job-service-account", cmdsupport.EnvOr(cleanupJobServiceAccountEnv, ""), "ServiceAccountName used by cleanup Jobs.")
	flags.StringVar(&cleanupJobEnvPassThrough, "cleanup-job-env-pass-through", cmdsupport.EnvOr(cleanupJobEnvPassThroughEnv, defaultCleanupPassThrough), "Comma-separated list of controller env vars copied into cleanup Jobs.")
	flags.StringVar(&publicationWorkerImage, "publication-worker-image", cmdsupport.EnvOr(publicationWorkerImageEnv, cmdsupport.EnvOr(cleanupJobImageEnv, "")), "Runtime image used for publication worker Pods.")
	flags.StringVar(&publicationWorkerImagePullSecretName, "publication-worker-image-pull-secret-name", cmdsupport.EnvOr(publicationWorkerImagePullSecretEnv, cmdsupport.EnvOr(cleanupJobImagePullSecretEnv, "")), "Optional imagePullSecret name used by publication worker Pods.")
	flags.StringVar(&publicationWorkerNamespace, "publication-worker-namespace", cmdsupport.EnvOr(publicationWorkerNamespaceEnv, cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models"))), "Namespace where publication worker Pods are created.")
	flags.StringVar(&publicationWorkerServiceAccount, "publication-worker-service-account", cmdsupport.EnvOr(publicationWorkerServiceAccountEnv, cmdsupport.EnvOr(cleanupJobServiceAccountEnv, "")), "ServiceAccountName used by publication worker Pods.")
	flags.StringVar(&publicationOCIRepositoryPrefix, "publication-oci-repository-prefix", cmdsupport.EnvOr(publicationOCIRepositoryPrefixEnv, ""), "OCI repository prefix used by publication workers.")
	flags.BoolVar(&publicationOCIInsecure, "publication-oci-insecure", cmdsupport.EnvOrBool(publicationOCIInsecureEnv, false), "Disable TLS verification for publication worker OCI registry access.")
	flags.StringVar(&publicationOCISecretName, "publication-oci-credentials-secret-name", cmdsupport.EnvOr(publicationOCISecretEnv, ""), "Secret with OCI registry username/password for publication workers.")
	flags.StringVar(&publicationOCICASecretName, "publication-oci-ca-secret-name", cmdsupport.EnvOr(publicationOCICASecretEnv, ""), "Optional Secret with ca.crt for publication worker OCI registry trust.")
	flags.IntVar(&publicationMaxConcurrentWorkers, "publication-max-concurrent-workers", cmdsupport.EnvOrInt(publicationMaxConcurrentWorkersEnv, defaultPublicationMaxConcurrentWorkers), "Maximum number of active publication worker Pods.")
	flags.StringVar(&publicationWorkVolumeType, "publication-work-volume-type", cmdsupport.EnvOr(publicationWorkVolumeTypeEnv, string(workloadpod.WorkVolumeTypeEmptyDir)), "Publication work volume type: EmptyDir or PersistentVolumeClaim.")
	flags.StringVar(&publicationWorkVolumeSizeLimit, "publication-work-volume-size-limit", cmdsupport.EnvOr(publicationWorkVolumeSizeLimitEnv, defaultPublicationWorkVolumeSizeLimit), "Bounded EmptyDir size limit for publication work volume.")
	flags.StringVar(&publicationWorkVolumeClaimName, "publication-work-volume-claim-name", cmdsupport.EnvOr(publicationWorkVolumeClaimNameEnv, defaultPublicationWorkVolumeClaimName), "PersistentVolumeClaim name used as publication work volume when PVC mode is enabled.")
	flags.StringVar(&publicationWorkerCPURequest, "publication-worker-cpu-request", cmdsupport.EnvOr(publicationWorkerCPURequestEnv, defaultPublicationWorkerCPURequest), "CPU request for publication worker Pods.")
	flags.StringVar(&publicationWorkerCPULimit, "publication-worker-cpu-limit", cmdsupport.EnvOr(publicationWorkerCPULimitEnv, defaultPublicationWorkerCPULimit), "CPU limit for publication worker Pods.")
	flags.StringVar(&publicationWorkerMemoryRequest, "publication-worker-memory-request", cmdsupport.EnvOr(publicationWorkerMemoryRequestEnv, defaultPublicationWorkerMemoryRequest), "Memory request for publication worker Pods.")
	flags.StringVar(&publicationWorkerMemoryLimit, "publication-worker-memory-limit", cmdsupport.EnvOr(publicationWorkerMemoryLimitEnv, defaultPublicationWorkerMemoryLimit), "Memory limit for publication worker Pods.")
	flags.StringVar(&publicationWorkerEphemeralRequest, "publication-worker-ephemeral-storage-request", cmdsupport.EnvOr(publicationWorkerEphemeralReqEnv, defaultPublicationWorkerEphemeralLimit), "Ephemeral-storage request for publication worker Pods.")
	flags.StringVar(&publicationWorkerEphemeralLimit, "publication-worker-ephemeral-storage-limit", cmdsupport.EnvOr(publicationWorkerEphemeralLimitEnv, defaultPublicationWorkerEphemeralLimit), "Ephemeral-storage limit for publication worker Pods.")
	flags.StringVar(&artifactsBucket, "artifacts-bucket", cmdsupport.EnvOr(artifactsBucketEnv, ""), "Bucket used for upload staging.")
	flags.StringVar(&artifactsS3Endpoint, "artifacts-s3-endpoint-url", cmdsupport.EnvOr(artifactsS3EndpointEnv, ""), "S3-compatible endpoint used for upload staging.")
	flags.StringVar(&artifactsS3Region, "artifacts-s3-region", cmdsupport.EnvOr(artifactsS3RegionEnv, ""), "S3-compatible region used for upload staging.")
	flags.BoolVar(&artifactsS3UsePathStyle, "artifacts-s3-use-path-style", cmdsupport.EnvOrBool(artifactsS3UsePathStyleEnv, false), "Use path-style addressing for upload staging object storage.")
	flags.BoolVar(&artifactsS3IgnoreTLS, "artifacts-s3-ignore-tls", cmdsupport.EnvOrBool(artifactsS3IgnoreTLSEnv, false), "Disable TLS verification for upload staging object storage.")
	flags.StringVar(&artifactsCredentialsSecretName, "artifacts-credentials-secret-name", cmdsupport.EnvOr(artifactsCredentialsSecretEnv, ""), "Secret with object storage accessKey/secretKey for upload staging.")
	flags.StringVar(&artifactsCASecretName, "artifacts-ca-secret-name", cmdsupport.EnvOr(artifactsCASecretEnv, ""), "Optional Secret with ca.crt for upload staging object storage.")
	flags.StringVar(&uploadServiceName, "upload-service-name", cmdsupport.EnvOr(uploadServiceNameEnv, "ai-models-controller"), "Shared Service name used for upload gateway URLs.")
	flags.StringVar(&uploadPublicHost, "upload-public-host", cmdsupport.EnvOr(uploadPublicHostEnv, ""), "Public host used for upload session ingress URLs.")
	flags.StringVar(&metricsBindAddress, "metrics-bind-address", cmdsupport.EnvOr(metricsBindAddressEnv, ":8080"), "The address the metric endpoint binds to.")
	flags.StringVar(&healthProbeBindAddress, "health-probe-bind-address", cmdsupport.EnvOr(healthProbeBindAddressEnv, ":8081"), "The address the health probe endpoint binds to.")
	flags.BoolVar(&leaderElect, "leader-elect", cmdsupport.EnvOrBool(leaderElectEnv, true), "Enable leader election for controller manager.")
	flags.StringVar(&leaderElectionID, "leader-election-id", cmdsupport.EnvOr(leaderElectionIDEnv, "ai-models-controller.deckhouse.io"), "Leader election ID used for controller manager leases.")
	flags.StringVar(&leaderElectionNamespace, "leader-election-namespace", cmdsupport.EnvOr(leaderElectionNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")), "Namespace used for leader election leases.")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	logger, err := cmdsupport.NewComponentLogger(logFormat, "controller")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai-models-controller: %v\n", err)
		return 1
	}
	cmdsupport.SetDefaultLogger(logger)

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	artifactsObjectStorage := objectstorage.Options{
		Bucket:                artifactsBucket,
		EndpointURL:           artifactsS3Endpoint,
		Region:                artifactsS3Region,
		UsePathStyle:          artifactsS3UsePathStyle,
		Insecure:              artifactsS3IgnoreTLS,
		CredentialsSecretName: artifactsCredentialsSecretName,
		CASecretName:          artifactsCASecretName,
	}

	workVolumeType, err := parsePublicationWorkVolumeType(publicationWorkVolumeType)
	if err != nil {
		logger.Error("invalid publication work volume type", slog.Any("error", err))
		return 1
	}
	workVolumeSizeLimit, err := parsePositiveQuantity("publication-work-volume-size-limit", publicationWorkVolumeSizeLimit)
	if err != nil {
		logger.Error("invalid publication work volume size limit", slog.Any("error", err))
		return 1
	}
	publicationWorkerResources, err := buildPublicationWorkerResources(
		publicationWorkerCPURequest,
		publicationWorkerCPULimit,
		publicationWorkerMemoryRequest,
		publicationWorkerMemoryLimit,
		publicationWorkerEphemeralRequest,
		publicationWorkerEphemeralLimit,
	)
	if err != nil {
		logger.Error("invalid publication worker resources", slog.Any("error", err))
		return 1
	}

	application, err := bootstrap.New(logger, bootstrap.Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:               cleanupJobNamespace,
				Image:                   cleanupJobImage,
				ImagePullSecretName:     cleanupJobImagePullSecretName,
				ServiceAccountName:      cleanupJobServiceAccount,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
				ObjectStorage:           artifactsObjectStorage,
				Env:                     cmdsupport.PassThroughEnv(cleanupJobEnvPassThrough),
			},
			RequeueAfter: 5 * time.Second,
		},
		PublicationRuntime: catalogstatus.Options{
			Runtime: catalogstatus.PublicationRuntimeOptions{
				Namespace:               publicationWorkerNamespace,
				Image:                   cmdsupport.FallbackString(publicationWorkerImage, cleanupJobImage),
				ImagePullSecretName:     cmdsupport.FallbackString(publicationWorkerImagePullSecretName, cleanupJobImagePullSecretName),
				ServiceAccountName:      cmdsupport.FallbackString(publicationWorkerServiceAccount, cleanupJobServiceAccount),
				OCIRepositoryPrefix:     publicationOCIRepositoryPrefix,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
				ObjectStorage:           artifactsObjectStorage,
				WorkVolume: workloadpod.WorkVolumeOptions{
					Type:                      workVolumeType,
					EmptyDirSizeLimit:         workVolumeSizeLimit,
					PersistentVolumeClaimName: publicationWorkVolumeClaimName,
				},
				Resources: publicationWorkerResources,
			},
			MaxConcurrentWorkers: publicationMaxConcurrentWorkers,
			UploadGateway: catalogstatus.UploadGatewayOptions{
				ServiceName: uploadServiceName,
				PublicHost:  uploadPublicHost,
			},
		},
		Runtime: bootstrap.RuntimeOptions{
			MetricsBindAddress:      metricsBindAddress,
			HealthProbeBindAddress:  healthProbeBindAddress,
			LeaderElection:          leaderElect,
			LeaderElectionID:        leaderElectionID,
			LeaderElectionNamespace: leaderElectionNamespace,
		},
	})
	if err != nil {
		logger.Error("unable to initialize controller app", slog.Any("error", err))
		return 1
	}

	if err := application.Run(ctx); err != nil {
		logger.Error("controller app exited with error", slog.Any("error", err))
		return 1
	}

	return 0
}

func parsePublicationWorkVolumeType(value string) (workloadpod.WorkVolumeType, error) {
	switch workloadpod.WorkVolumeType(value) {
	case workloadpod.WorkVolumeTypeEmptyDir, workloadpod.WorkVolumeTypePersistentVolumeClaim:
		return workloadpod.WorkVolumeType(value), nil
	default:
		return "", fmt.Errorf("unsupported publication work volume type %q", value)
	}
}

func parsePositiveQuantity(flagName, value string) (resource.Quantity, error) {
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("%s: %w", flagName, err)
	}
	if quantity.Sign() <= 0 {
		return resource.Quantity{}, fmt.Errorf("%s must be greater than zero", flagName)
	}
	return quantity, nil
}

func buildPublicationWorkerResources(
	cpuRequest string,
	cpuLimit string,
	memoryRequest string,
	memoryLimit string,
	ephemeralRequest string,
	ephemeralLimit string,
) (corev1.ResourceRequirements, error) {
	requestCPU, err := parsePositiveQuantity("publication-worker-cpu-request", cpuRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitCPU, err := parsePositiveQuantity("publication-worker-cpu-limit", cpuLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	requestMemory, err := parsePositiveQuantity("publication-worker-memory-request", memoryRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitMemory, err := parsePositiveQuantity("publication-worker-memory-limit", memoryLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	requestEphemeral, err := parsePositiveQuantity("publication-worker-ephemeral-storage-request", ephemeralRequest)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}
	limitEphemeral, err := parsePositiveQuantity("publication-worker-ephemeral-storage-limit", ephemeralLimit)
	if err != nil {
		return corev1.ResourceRequirements{}, err
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:              requestCPU,
			corev1.ResourceMemory:           requestMemory,
			corev1.ResourceEphemeralStorage: requestEphemeral,
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:              limitCPU,
			corev1.ResourceMemory:           limitMemory,
			corev1.ResourceEphemeralStorage: limitEphemeral,
		},
	}, nil
}
