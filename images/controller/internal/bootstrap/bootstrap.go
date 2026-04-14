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

	apiinstall "github.com/deckhouse/ai-models/api/core/install"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	"github.com/deckhouse/ai-models/controller/internal/monitoring/catalogmetrics"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type Options struct {
	CleanupJobs        catalogcleanup.Options
	PublicationRuntime catalogstatus.Options
	WorkloadDelivery   workloaddelivery.Options
	Runtime            RuntimeOptions
}

type RuntimeOptions struct {
	MetricsBindAddress      string
	HealthProbeBindAddress  string
	LeaderElection          bool
	LeaderElectionID        string
	LeaderElectionNamespace string
}

type App struct {
	logger             *slog.Logger
	cleanupJobs        catalogcleanup.Options
	publicationRuntime catalogstatus.Options
	workloadDelivery   workloaddelivery.Options
	runtime            RuntimeOptions
}

func New(logger *slog.Logger, options Options) (*App, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}
	if err := options.CleanupJobs.CleanupJob.Validate(); err != nil {
		return nil, err
	}
	if err := options.PublicationRuntime.Validate(); err != nil {
		return nil, err
	}
	if err := options.WorkloadDelivery.Validate(); err != nil {
		return nil, err
	}

	runtimeOptions := normalizeRuntimeOptions(options.Runtime)

	return &App{
		cleanupJobs:        options.CleanupJobs,
		publicationRuntime: options.PublicationRuntime,
		workloadDelivery:   options.WorkloadDelivery,
		logger:             logger,
		runtime:            runtimeOptions,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	scheme := runtime.NewScheme()
	apiinstall.Install(scheme)
	if err := appsv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		return err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: a.runtime.MetricsBindAddress,
		},
		HealthProbeBindAddress:        a.runtime.HealthProbeBindAddress,
		LeaderElection:                a.runtime.LeaderElection,
		LeaderElectionID:              a.runtime.LeaderElectionID,
		LeaderElectionNamespace:       a.runtime.LeaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		return err
	}

	if err := catalogcleanup.SetupWithManager(mgr, a.cleanupJobs); err != nil {
		return err
	}
	if err := catalogstatus.SetupWithManager(mgr, a.publicationRuntime); err != nil {
		return err
	}
	if err := workloaddelivery.SetupWithManager(mgr, a.workloadDelivery); err != nil {
		return err
	}
	catalogmetrics.SetupCollector(
		mgr.GetCache(),
		metrics.Registry,
		a.logger.With(slog.String("runtimeKind", "metrics")),
	)
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	a.logger.Info(
		"controller bootstrap ready",
		slog.String("metricsBindAddress", a.runtime.MetricsBindAddress),
		slog.String("healthProbeBindAddress", a.runtime.HealthProbeBindAddress),
		slog.Bool("leaderElection", a.runtime.LeaderElection),
		slog.String("leaderElectionID", a.runtime.LeaderElectionID),
		slog.String("leaderElectionNamespace", a.runtime.LeaderElectionNamespace),
		slog.String("cleanupJobNamespace", a.cleanupJobs.CleanupJob.Namespace),
		slog.String("cleanupJobImage", a.cleanupJobs.CleanupJob.Image),
	)
	if a.publicationRuntime.Enabled() {
		a.logger.Info(
			"controller model publication configured",
			slog.String("publicationRuntimeNamespace", a.publicationRuntime.Runtime.Namespace),
			slog.String("publicationRuntimeImage", a.publicationRuntime.Runtime.Image),
			slog.Int("publicationMaxConcurrentWorkers", a.publicationRuntime.MaxConcurrentWorkers),
			slog.String("publicationWorkVolumeType", string(a.publicationRuntime.Runtime.WorkVolume.Type)),
		)
	}
	if a.workloadDelivery.Enabled() {
		a.logger.Info(
			"controller workload delivery configured",
			slog.String("deliveryRuntimeImage", a.workloadDelivery.Service.Render.RuntimeImage),
			slog.String("deliveryRegistrySourceNamespace", a.workloadDelivery.Service.RegistrySourceNamespace),
		)
	}

	if err := mgr.Start(ctx); err != nil {
		return err
	}

	a.logger.Info("controller manager stopped")
	return nil
}

func normalizeRuntimeOptions(options RuntimeOptions) RuntimeOptions {
	if strings.TrimSpace(options.MetricsBindAddress) == "" {
		options.MetricsBindAddress = ":8080"
	}
	if strings.TrimSpace(options.HealthProbeBindAddress) == "" {
		options.HealthProbeBindAddress = ":8081"
	}
	if strings.TrimSpace(options.LeaderElectionID) == "" {
		options.LeaderElectionID = "ai-models-controller.deckhouse.io"
	}
	if strings.TrimSpace(options.LeaderElectionNamespace) == "" {
		options.LeaderElectionNamespace = "d8-ai-models"
	}

	return options
}
