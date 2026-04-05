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
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/cleanupjob"
	"github.com/deckhouse/ai-models/controller/internal/app"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/publicationops"
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

func run() int {
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

	flag.StringVar(&logFormat, "log-format", envOr(logFormatEnv, "text"), "Log format: text or json.")
	flag.StringVar(&cleanupJobImage, "cleanup-job-image", envOr(cleanupJobImageEnv, ""), "Backend image used for cleanup Jobs.")
	flag.StringVar(&cleanupJobNamespace, "cleanup-job-namespace", envOr(cleanupJobNamespaceEnv, envOr("POD_NAMESPACE", "d8-ai-models")), "Namespace where cleanup Jobs are created.")
	flag.StringVar(&cleanupJobServiceAccount, "cleanup-job-service-account", envOr(cleanupJobServiceAccountEnv, ""), "ServiceAccountName used by cleanup Jobs.")
	flag.StringVar(&cleanupJobEnvPassThrough, "cleanup-job-env-pass-through", envOr(cleanupJobEnvPassThroughEnv, defaultCleanupPassThrough), "Comma-separated list of controller env vars copied into cleanup Jobs.")
	flag.StringVar(&publicationWorkerImage, "publication-worker-image", envOr(publicationWorkerImageEnv, envOr(cleanupJobImageEnv, "")), "Backend image used for publication worker Pods.")
	flag.StringVar(&publicationWorkerNamespace, "publication-worker-namespace", envOr(publicationWorkerNamespaceEnv, envOr(cleanupJobNamespaceEnv, envOr("POD_NAMESPACE", "d8-ai-models"))), "Namespace where publication worker Pods and operation state are created.")
	flag.StringVar(&publicationWorkerServiceAccount, "publication-worker-service-account", envOr(publicationWorkerServiceAccountEnv, envOr(cleanupJobServiceAccountEnv, "")), "ServiceAccountName used by publication worker Pods.")
	flag.StringVar(&publicationOCIRepositoryPrefix, "publication-oci-repository-prefix", envOr(publicationOCIRepositoryPrefixEnv, ""), "OCI repository prefix used by publication workers.")
	flag.BoolVar(&publicationOCIInsecure, "publication-oci-insecure", envOrBool(publicationOCIInsecureEnv, false), "Disable TLS verification for publication worker OCI registry access.")
	flag.StringVar(&publicationOCISecretName, "publication-oci-credentials-secret-name", envOr(publicationOCISecretEnv, ""), "Secret with OCI registry username/password for publication workers.")
	flag.StringVar(&publicationOCICASecretName, "publication-oci-ca-secret-name", envOr(publicationOCICASecretEnv, ""), "Optional Secret with ca.crt for publication worker OCI registry trust.")
	flag.StringVar(&metricsBindAddress, "metrics-bind-address", envOr(metricsBindAddressEnv, ":8080"), "The address the metric endpoint binds to.")
	flag.StringVar(&healthProbeBindAddress, "health-probe-bind-address", envOr(healthProbeBindAddressEnv, ":8081"), "The address the health probe endpoint binds to.")
	flag.BoolVar(&leaderElect, "leader-elect", envOrBool(leaderElectEnv, true), "Enable leader election for controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", envOr(leaderElectionIDEnv, "ai-models-controller.deckhouse.io"), "Leader election ID used for controller manager leases.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", envOr(leaderElectionNamespaceEnv, envOr("POD_NAMESPACE", "d8-ai-models")), "Namespace used for leader election leases.")
	flag.Parse()

	logger, err := newLogger(logFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ai-models-controller: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(logger, app.Options{
		CleanupJobs: catalogcleanup.Options{
			CleanupJob: cleanupjob.Options{
				Namespace:               cleanupJobNamespace,
				Image:                   cleanupJobImage,
				ServiceAccountName:      cleanupJobServiceAccount,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
				Env:                     passThroughEnv(cleanupJobEnvPassThrough),
			},
			RequeueAfter: 5 * time.Second,
		},
		HFPublication: catalogstatus.Options{
			OperationNamespace: publicationWorkerNamespace,
			RequeueAfter:       5 * time.Second,
		},
		PublicationOps: publicationops.Options{
			PublishPod: publicationops.PublishPodOptions{
				Namespace:               publicationWorkerNamespace,
				Image:                   fallbackString(publicationWorkerImage, cleanupJobImage),
				ServiceAccountName:      fallbackString(publicationWorkerServiceAccount, cleanupJobServiceAccount),
				OCIRepositoryPrefix:     publicationOCIRepositoryPrefix,
				OCIInsecure:             publicationOCIInsecure,
				OCIRegistrySecretName:   publicationOCISecretName,
				OCIRegistryCASecretName: publicationOCICASecretName,
			},
			RequeueAfter: 5 * time.Second,
		},
		Runtime: app.RuntimeOptions{
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

func newLogger(format string) (*slog.Logger, error) {
	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(os.Stderr, nil)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stderr, nil)), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func envOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok && value != "" {
		return value
	}

	return fallback
}

func envOrBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func passThroughEnv(csv string) []corev1.EnvVar {
	names := strings.Split(csv, ",")
	result := make([]corev1.EnvVar, 0, len(names))
	seen := map[string]struct{}{}

	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, corev1.EnvVar{Name: name, Value: value})
	}

	return result
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}

	return fallback
}
