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

package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	apiinstall "github.com/deckhouse/ai-models/api/core/install"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/controllers/nodecachesubstrate"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	"github.com/deckhouse/ai-models/controller/internal/monitoring/catalogmetrics"
	"github.com/deckhouse/ai-models/controller/internal/monitoring/runtimehealth"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

const defaultControllerCacheSyncTimeout = 10 * time.Minute
const defaultWebhookCertDir = "/tmp/k8s-webhook-server/serving-certs"
const defaultWebhookPort = 9443

type Options struct {
	Cleanup            catalogcleanup.Options
	PublicationRuntime catalogstatus.Options
	NodeCacheRuntime   nodecacheruntime.Options
	NodeCacheSubstrate nodecachesubstrate.Options
	WorkloadDelivery   workloaddelivery.Options
	Runtime            RuntimeOptions
}

type RuntimeOptions struct {
	MetricsBindAddress      string
	HealthProbeBindAddress  string
	WebhookCertDir          string
	WebhookPort             int
	LeaderElection          bool
	LeaderElectionID        string
	LeaderElectionNamespace string
}

type App struct {
	logger             *slog.Logger
	cleanup            catalogcleanup.Options
	publicationRuntime catalogstatus.Options
	nodeCacheRuntime   nodecacheruntime.Options
	nodeCacheSubstrate nodecachesubstrate.Options
	workloadDelivery   workloaddelivery.Options
	runtime            RuntimeOptions
}

func New(logger *slog.Logger, options Options) (*App, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}
	if err := options.Cleanup.Cleanup.Validate(); err != nil {
		return nil, err
	}
	if err := options.PublicationRuntime.Validate(); err != nil {
		return nil, err
	}
	if err := options.NodeCacheRuntime.Validate(); err != nil {
		return nil, err
	}
	if err := options.NodeCacheSubstrate.Validate(); err != nil {
		return nil, err
	}
	if err := options.WorkloadDelivery.Validate(); err != nil {
		return nil, err
	}

	runtimeOptions := normalizeRuntimeOptions(options.Runtime)

	return &App{
		cleanup:            options.Cleanup,
		publicationRuntime: options.PublicationRuntime,
		nodeCacheRuntime:   options.NodeCacheRuntime,
		nodeCacheSubstrate: options.NodeCacheSubstrate,
		workloadDelivery:   options.WorkloadDelivery,
		logger:             logger,
		runtime:            runtimeOptions,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	scheme, err := buildManagerScheme()
	if err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), managerOptions(scheme, a.logger, a.runtime))
	if err != nil {
		return err
	}

	if err := a.setupManager(mgr); err != nil {
		return err
	}

	a.logRuntimeConfiguration()

	if err := mgr.Start(ctx); err != nil {
		return err
	}

	a.logger.Info("controller manager stopped")
	return nil
}

func buildManagerScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	apiinstall.Install(scheme)
	if err := appsv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}

func (a *App) setupManager(mgr ctrl.Manager) error {
	if err := catalogcleanup.SetupWithManager(mgr, a.cleanup); err != nil {
		return err
	}
	if err := catalogstatus.SetupWithManager(mgr, a.publicationRuntime); err != nil {
		return err
	}
	if err := nodecacheruntime.SetupWithManager(mgr, a.logger, a.nodeCacheRuntime); err != nil {
		return err
	}
	if err := nodecachesubstrate.SetupWithManager(mgr, a.logger, a.nodeCacheSubstrate); err != nil {
		return err
	}
	workloadDeliveryEnabled := a.workloadDelivery.Enabled()
	if err := workloaddelivery.SetupWithManager(mgr, a.workloadDelivery); err != nil {
		return err
	}

	catalogmetrics.SetupCollector(
		mgr.GetCache(),
		metrics.Registry,
		a.logger.With(slog.String("runtimeKind", "metrics")),
	)
	if a.nodeCacheRuntime.Enabled {
		runtimehealth.SetupCollector(
			mgr.GetCache(),
			metrics.Registry,
			a.logger.With(slog.String("runtimeKind", "metrics")),
			runtimehealth.Options{
				RuntimeNamespace:   a.nodeCacheRuntime.Namespace,
				NodeSelectorLabels: a.nodeCacheRuntime.NodeSelectorLabels,
			},
		)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}
	if workloadDeliveryEnabled {
		if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
			return err
		}
		if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) logRuntimeConfiguration() {
	a.logger.Info(
		"controller bootstrap ready",
		slog.String("metricsBindAddress", a.runtime.MetricsBindAddress),
		slog.String("healthProbeBindAddress", a.runtime.HealthProbeBindAddress),
		slog.String("webhookCertDir", a.runtime.WebhookCertDir),
		slog.Int("webhookPort", a.runtime.WebhookPort),
		slog.Bool("leaderElection", a.runtime.LeaderElection),
		slog.String("leaderElectionID", a.runtime.LeaderElectionID),
		slog.String("leaderElectionNamespace", a.runtime.LeaderElectionNamespace),
		slog.String("cleanupNamespace", a.cleanup.Cleanup.Namespace),
	)
	if a.publicationRuntime.Enabled() {
		a.logger.Info(
			"controller model publication configured",
			slog.String("publicationRuntimeNamespace", a.publicationRuntime.Runtime.Namespace),
			slog.String("publicationRuntimeImage", a.publicationRuntime.Runtime.Image),
			slog.Int("publicationMaxConcurrentWorkers", a.publicationRuntime.MaxConcurrentWorkers),
		)
	}
	if a.workloadDelivery.Enabled() {
		a.logger.Info(
			"controller workload delivery configured",
			slog.String("deliveryRuntimeImage", a.workloadDelivery.Service.Render.RuntimeImage),
			slog.String("deliveryRegistrySourceNamespace", a.workloadDelivery.Service.RegistrySourceNamespace),
		)
	}
	if a.nodeCacheSubstrate.Enabled {
		a.logger.Info(
			"controller node-cache substrate configured",
			slog.String("storageClassName", a.nodeCacheSubstrate.StorageClassName),
			slog.String("volumeGroupSetName", a.nodeCacheSubstrate.VolumeGroupSetName),
			slog.String("maxSize", a.nodeCacheSubstrate.MaxSize),
		)
	}
	if a.nodeCacheRuntime.Enabled {
		a.logger.Info(
			"controller node-cache runtime configured",
			slog.String("namespace", a.nodeCacheRuntime.Namespace),
			slog.String("storageClassName", a.nodeCacheRuntime.StorageClassName),
			slog.String("sharedVolumeSize", a.nodeCacheRuntime.SharedVolumeSize),
		)
	}
}

func normalizeRuntimeOptions(options RuntimeOptions) RuntimeOptions {
	if strings.TrimSpace(options.MetricsBindAddress) == "" {
		options.MetricsBindAddress = ":8080"
	}
	if strings.TrimSpace(options.HealthProbeBindAddress) == "" {
		options.HealthProbeBindAddress = ":8081"
	}
	if strings.TrimSpace(options.WebhookCertDir) == "" {
		options.WebhookCertDir = defaultWebhookCertDir
	}
	if options.WebhookPort == 0 {
		options.WebhookPort = defaultWebhookPort
	}
	if strings.TrimSpace(options.LeaderElectionID) == "" {
		options.LeaderElectionID = "ai-models-controller.deckhouse.io"
	}
	if strings.TrimSpace(options.LeaderElectionNamespace) == "" {
		options.LeaderElectionNamespace = "d8-ai-models"
	}

	return options
}

func managerOptions(scheme *runtime.Scheme, logger *slog.Logger, runtimeOptions RuntimeOptions) ctrl.Options {
	if logger == nil {
		logger = slog.Default()
	}
	bridged := logr.FromSlogHandler(logger.Handler())

	return ctrl.Options{
		Scheme: scheme,
		Logger: bridged,
		Metrics: metricsserver.Options{
			BindAddress: runtimeOptions.MetricsBindAddress,
		},
		HealthProbeBindAddress:        runtimeOptions.HealthProbeBindAddress,
		LeaderElection:                runtimeOptions.LeaderElection,
		LeaderElectionID:              runtimeOptions.LeaderElectionID,
		LeaderElectionNamespace:       runtimeOptions.LeaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    runtimeOptions.WebhookPort,
			CertDir: runtimeOptions.WebhookCertDir,
		}),
		Controller: config.Controller{
			CacheSyncTimeout: defaultControllerCacheSyncTimeout,
			RecoverPanic:     ptr.To(true),
			UsePriorityQueue: ptr.To(true),
			Logger:           bridged,
		},
	}
}
