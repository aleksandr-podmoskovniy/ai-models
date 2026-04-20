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

package nodecacheruntime

import (
	"context"
	"log/slog"
	"strings"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const controllerName = "node-cache-runtime"

func SetupWithManager(mgr ctrl.Manager, logger *slog.Logger, options Options) error {
	if !options.Enabled {
		return nil
	}
	if err := options.Validate(); err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}
	service, err := k8sadapters.NewService(mgr.GetClient(), mgr.GetScheme())
	if err != nil {
		return err
	}
	reconciler := &Reconciler{
		client:  mgr.GetClient(),
		service: service,
		logger:  logger.With(slog.String("controller", controllerName)),
		options: options,
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&corev1.Node{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapRuntimeObjectToNode), builder.WithPredicates(predicate.NewPredicateFuncs(isManagedRuntimeObject))).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(mapRuntimeObjectToNode), builder.WithPredicates(predicate.NewPredicateFuncs(isManagedRuntimeObject))).
		Complete(reconciler)
}

func mapRuntimeObjectToNode(_ context.Context, object client.Object) []reconcile.Request {
	nodeName := strings.TrimSpace(object.GetAnnotations()[k8sadapters.NodeNameAnnotationKey])
	if nodeName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: nodeName}}}
}

func isManagedRuntimeObject(object client.Object) bool {
	return object.GetLabels()[k8sadapters.ManagedLabelKey] == k8sadapters.ManagedLabelValue
}
