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

	"github.com/deckhouse/ai-models/controller/internal/bootstrap"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
)

const (
	logFormatEnv                       = "LOG_FORMAT"
	cleanupJobImageEnv                 = "CLEANUP_JOB_IMAGE"
	cleanupJobNamespaceEnv             = "CLEANUP_JOB_NAMESPACE"
	cleanupJobServiceAccountEnv        = "CLEANUP_JOB_SERVICE_ACCOUNT"
	cleanupJobEnvPassThroughEnv        = "CLEANUP_JOB_ENV_PASS_THROUGH"
	publicationWorkerImageEnv          = "PUBLICATION_WORKER_IMAGE"
	publicationWorkerNamespaceEnv      = "PUBLICATION_WORKER_NAMESPACE"
	publicationWorkerServiceAccountEnv = "PUBLICATION_WORKER_SERVICE_ACCOUNT"
	publicationOCIRepositoryPrefixEnv  = "PUBLICATION_OCI_REPOSITORY_PREFIX"
	publicationOCIInsecureEnv          = "PUBLICATION_OCI_INSECURE"
	publicationOCISecretEnv            = "PUBLICATION_OCI_CREDENTIALS_SECRET_NAME"
	publicationOCICASecretEnv          = "PUBLICATION_OCI_CA_SECRET_NAME"
	metricsBindAddressEnv              = "METRICS_BIND_ADDRESS"
	healthProbeBindAddressEnv          = "HEALTH_PROBE_BIND_ADDRESS"
	leaderElectEnv                     = "LEADER_ELECT"
	leaderElectionIDEnv                = "LEADER_ELECTION_ID"
	leaderElectionNamespaceEnv         = "LEADER_ELECTION_NAMESPACE"
)

const defaultCleanupPassThrough = "SSL_CERT_FILE,REQUESTS_CA_BUNDLE,AWS_CA_BUNDLE"

func runManager(args []string) int {
	var logFormat string
	var cleanupJobImage string
	var cleanupJobNamespace string
	var cleanupJobServiceAccount string
	var cleanupJobEnvPassThrough string
	var publicationWorkerImage string
	var publicationWorkerNamespace string
	var publicationWorkerServiceAccount string
	var publicationOCIRepositoryPrefix string
	var publicationOCIInsecure bool
	var publicationOCISecretName string
	var publicationOCICASecretName string
	var metricsBindAddress string
	var healthProbeBindAddress string
	var leaderElect bool
	var leaderElectionID string
	var leaderElectionNamespace string

	flags := cmdsupport.NewFlagSet("ai-models-controller")
	flags.StringVar(&logFormat, "log-format", cmdsupport.EnvOr(logFormatEnv, "text"), "Log format: text or json.")
	flags.StringVar(&cleanupJobImage, "cleanup-job-image", cmdsupport.EnvOr(cleanupJobImageEnv, ""), "Runtime image used for cleanup Jobs.")
	flags.StringVar(&cleanupJobNamespace, "cleanup-job-namespace", cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")), "Namespace where cleanup Jobs are created.")
	flags.StringVar(&cleanupJobServiceAccount, "cleanup-job-service-account", cmdsupport.EnvOr(cleanupJobServiceAccountEnv, ""), "ServiceAccountName used by cleanup Jobs.")
	flags.StringVar(&cleanupJobEnvPassThrough, "cleanup-job-env-pass-through", cmdsupport.EnvOr(cleanupJobEnvPassThroughEnv, defaultCleanupPassThrough), "Comma-separated list of controller env vars copied into cleanup Jobs.")
	flags.StringVar(&publicationWorkerImage, "publication-worker-image", cmdsupport.EnvOr(publicationWorkerImageEnv, cmdsupport.EnvOr(cleanupJobImageEnv, "")), "Runtime image used for publication worker Pods.")
	flags.StringVar(&publicationWorkerNamespace, "publication-worker-namespace", cmdsupport.EnvOr(publicationWorkerNamespaceEnv, cmdsupport.EnvOr(cleanupJobNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models"))), "Namespace where publication worker Pods are created.")
	flags.StringVar(&publicationWorkerServiceAccount, "publication-worker-service-account", cmdsupport.EnvOr(publicationWorkerServiceAccountEnv, cmdsupport.EnvOr(cleanupJobServiceAccountEnv, "")), "ServiceAccountName used by publication worker Pods.")
	flags.StringVar(&publicationOCIRepositoryPrefix, "publication-oci-repository-prefix", cmdsupport.EnvOr(publicationOCIRepositoryPrefixEnv, ""), "OCI repository prefix used by publication workers.")
	flags.BoolVar(&publicationOCIInsecure, "publication-oci-insecure", cmdsupport.EnvOrBool(publicationOCIInsecureEnv, false), "Disable TLS verification for publication worker OCI registry access.")
	flags.StringVar(&publicationOCISecretName, "publication-oci-credentials-secret-name", cmdsupport.EnvOr(publicationOCISecretEnv, ""), "Secret with OCI registry username/password for publication workers.")
	flags.StringVar(&publicationOCICASecretName, "publication-oci-ca-secret-name", cmdsupport.EnvOr(publicationOCICASecretEnv, ""), "Optional Secret with ca.crt for publication worker OCI registry trust.")
	flags.StringVar(&metricsBindAddress, "metrics-bind-address", cmdsupport.EnvOr(metricsBindAddressEnv, ":8080"), "The address the metric endpoint binds to.")
	flags.StringVar(&healthProbeBindAddress, "health-probe-bind-address", cmdsupport.EnvOr(healthProbeBindAddressEnv, ":8081"), "The address the health probe endpoint binds to.")
	flags.BoolVar(&leaderElect, "leader-elect", cmdsupport.EnvOrBool(leaderElectEnv, true), "Enable leader election for controller manager.")
	flags.StringVar(&leaderElectionID, "leader-election-id", cmdsupport.EnvOr(leaderElectionIDEnv, "ai-models-controller.deckhouse.io"), "Leader election ID used for controller manager leases.")
	flags.StringVar(&leaderElectionNamespace, "leader-election-namespace", cmdsupport.EnvOr(leaderElectionNamespaceEnv, cmdsupport.EnvOr("POD_NAMESPACE", "d8-ai-models")), "Namespace used for leader election leases.")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	logger, err := cmdsupport.NewLogger(logFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai-models-controller: %v\n", err)
		return 1
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	application, err := bootstrap.New(logger, bootstrap.Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: catalogcleanup.CleanupJobOptions{
				Namespace:               cleanupJobNamespace,
				Image:                   cleanupJobImage,
				ServiceAccountName:      cleanupJobServiceAccount,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
				Env:                     cmdsupport.PassThroughEnv(cleanupJobEnvPassThrough),
			},
			RequeueAfter: 5 * time.Second,
		},
		PublicationRuntime: catalogstatus.Options{
			Runtime: catalogstatus.PublicationRuntimeOptions{
				Namespace:               publicationWorkerNamespace,
				Image:                   cmdsupport.FallbackString(publicationWorkerImage, cleanupJobImage),
				ServiceAccountName:      cmdsupport.FallbackString(publicationWorkerServiceAccount, cleanupJobServiceAccount),
				OCIRepositoryPrefix:     publicationOCIRepositoryPrefix,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
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
