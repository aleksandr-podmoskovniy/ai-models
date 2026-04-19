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

package nodecacheintent

import (
	"context"
	"log/slog"
	"strings"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheintent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	controllerName        = "node-cache-intent"
	podNodeNameIndexField = "ai.deckhouse.io/nodecacheintent-pod-node-name"
)

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
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, podNodeNameIndexField, podNodeNameIndexValue); err != nil {
		return err
	}
	service, err := k8sadapters.NewService(mgr.GetClient())
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
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(mapPodToNode), builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(mapConfigMapToNode), builder.WithPredicates(predicate.NewPredicateFuncs(isManagedIntentConfigMap))).
		Complete(reconciler)
}

func podNodeNameIndexValue(object client.Object) []string {
	pod, ok := object.(*corev1.Pod)
	if !ok {
		return nil
	}
	if nodeName := strings.TrimSpace(pod.Spec.NodeName); nodeName != "" {
		return []string{nodeName}
	}
	return nil
}

func mapPodToNode(_ context.Context, object client.Object) []reconcile.Request {
	pod, ok := object.(*corev1.Pod)
	if !ok {
		return nil
	}
	nodeName := strings.TrimSpace(pod.Spec.NodeName)
	if nodeName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: nodeName}}}
}

func mapConfigMapToNode(_ context.Context, object client.Object) []reconcile.Request {
	configMap, ok := object.(*corev1.ConfigMap)
	if !ok {
		return nil
	}
	nodeName := strings.TrimSpace(configMap.Annotations[k8sadapters.NodeNameAnnotationKey])
	if nodeName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: nodeName}}}
}

func isManagedIntentConfigMap(object client.Object) bool {
	configMap, ok := object.(*corev1.ConfigMap)
	if !ok {
		return false
	}
	return configMap.Labels[k8sadapters.ManagedLabelKey] == k8sadapters.ManagedLabelValue
}
