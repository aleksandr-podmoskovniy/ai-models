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

package catalogstatus

import (
	"errors"
	"log/slog"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/auditevent"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type Options struct {
	Runtime              PublicationRuntimeOptions
	MaxConcurrentWorkers int
	UploadGateway        UploadGatewayOptions
}

type PublicationRuntimeOptions = workloadpod.RuntimeOptions
type UploadGatewayOptions = uploadsession.GatewayOptions

const (
	modelControllerName        = "catalogstatus-model"
	clusterModelControllerName = "catalogstatus-cluster-model"
)

type baseReconciler struct {
	client         client.Client
	options        Options
	sourceWorkers  publicationports.SourceWorkerRuntime
	uploadSessions publicationports.UploadSessionRuntime
	auditSink      auditsink.Sink
}

type ModelReconciler struct{ baseReconciler }
type ClusterModelReconciler struct{ baseReconciler }

func SetupWithManager(mgr ctrl.Manager, options Options) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}
	if !options.Enabled() {
		return nil
	}

	if err := options.Validate(); err != nil {
		return err
	}

	sourceWorkers, err := sourceworker.NewService(
		mgr.GetClient(),
		mgr.GetScheme(),
		sourceWorkerOptions(options.Runtime, options.MaxConcurrentWorkers),
	)
	if err != nil {
		return err
	}
	uploadSessions, err := uploadsession.NewService(mgr.GetClient(), mgr.GetScheme(), uploadSessionOptions(options.Runtime, options.UploadGateway))
	if err != nil {
		return err
	}
	auditRecorder, err := auditevent.New(
		mgr.GetEventRecorderFor(modelControllerName),
		slog.Default().With(
			slog.String("controller", "catalogstatus"),
			slog.String("runtimeKind", "audit"),
		),
	)
	if err != nil {
		return err
	}

	base := baseReconciler{
		client:         mgr.GetClient(),
		options:        options,
		sourceWorkers:  sourceWorkers,
		uploadSessions: uploadSessions,
		auditSink:      auditRecorder,
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		Named(modelControllerName).
		For(&modelsv1alpha1.Model{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodToModelRequests)).
		Complete(&ModelReconciler{base}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(clusterModelControllerName).
		For(&modelsv1alpha1.ClusterModel{}).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodToClusterModelRequests)).
		Complete(&ClusterModelReconciler{base})
}

func (o Options) Enabled() bool {
	return strings.TrimSpace(o.Runtime.Namespace) != "" &&
		strings.TrimSpace(o.Runtime.Image) != ""
}

func (o Options) Validate() error {
	if !o.Enabled() {
		return nil
	}
	if err := workloadpod.ValidateRuntimeOptions("publication runtime", o.Runtime); err != nil {
		return err
	}
	if o.MaxConcurrentWorkers <= 0 {
		return errors.New("publication runtime max concurrent workers must be greater than zero")
	}
	return objectstorage.ValidateOptions("publication runtime", o.Runtime.ObjectStorage)
}

func sourceWorkerOptions(o PublicationRuntimeOptions, maxConcurrentWorkers int) sourceworker.Options {
	return sourceworker.Options{
		RuntimeOptions:       o,
		MaxConcurrentWorkers: maxConcurrentWorkers,
	}
}

func uploadSessionOptions(o PublicationRuntimeOptions, gateway uploadsession.GatewayOptions) uploadsession.Options {
	return uploadsession.Options{
		Runtime: uploadsession.RuntimeOptions{
			Namespace:           o.Namespace,
			OCIRepositoryPrefix: o.OCIRepositoryPrefix,
		},
		Gateway: gateway,
	}
}
