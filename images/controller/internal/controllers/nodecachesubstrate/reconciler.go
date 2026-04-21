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

package nodecachesubstrate

import (
	"context"
	"log/slog"
	"reflect"
	"time"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecachesubstrate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const requeueAfterNoReadyLVGs = 30 * time.Second

type Reconciler struct {
	client  client.Client
	logger  *slog.Logger
	options Options
}

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
	reconciler := &Reconciler{
		client:  mgr.GetClient(),
		logger:  logger.With(slog.String("controller", "node-cache-substrate")),
		options: options,
	}

	mapToSingleton := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "managed"}}}
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("node-cache-substrate").
		For(&corev1.Node{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(k8sadapters.NewLVMVolumeGroupSet(""), mapToSingleton, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(k8sadapters.NewLVMVolumeGroup(""), mapToSingleton, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(k8sadapters.NewLocalStorageClass(""), mapToSingleton, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Complete(reconciler)
}

func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	if !r.options.Enabled {
		return ctrl.Result{}, nil
	}
	if err := r.ensureVolumeGroupSet(ctx); err != nil {
		return ctrl.Result{}, err
	}

	lvgNames, err := r.readyManagedLVMVolumeGroups(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(lvgNames) == 0 {
		r.logger.Info("managed node-cache substrate waits for ready LVMVolumeGroups")
		return ctrl.Result{RequeueAfter: requeueAfterNoReadyLVGs}, nil
	}
	if err := r.ensureLocalStorageClass(ctx, lvgNames); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info(
		"managed node-cache substrate reconciled",
		slog.String("storageClassName", r.options.StorageClassName),
		slog.String("volumeGroupSetName", r.options.VolumeGroupSetName),
		slog.Int("readyVolumeGroups", len(lvgNames)),
	)
	return ctrl.Result{}, nil
}

func (r *Reconciler) ensureVolumeGroupSet(ctx context.Context) error {
	desired := k8sadapters.DesiredLVMVolumeGroupSet(k8sadapters.LVMVolumeGroupSetSpec{
		Name:                   r.options.VolumeGroupSetName,
		MaxSize:                r.options.MaxSize,
		ThinPoolName:           r.options.ThinPoolName,
		VolumeGroupNameOnNode:  r.options.VolumeGroupNameOnNode,
		NodeSelectorLabels:     r.options.NodeSelectorLabels,
		BlockDeviceMatchLabels: r.options.BlockDeviceMatchLabels,
	})
	return r.syncObject(ctx, k8sadapters.NewLVMVolumeGroupSet(r.options.VolumeGroupSetName), desired)
}

func (r *Reconciler) readyManagedLVMVolumeGroups(ctx context.Context) ([]string, error) {
	list := k8sadapters.NewLVMVolumeGroupList()
	if err := r.client.List(ctx, list, client.MatchingLabels{
		k8sadapters.ManagedLabelKey: k8sadapters.ManagedLabelValue,
	}); err != nil {
		return nil, err
	}
	return k8sadapters.ReadyManagedLVMVolumeGroupNames(list.Items), nil
}

func (r *Reconciler) ensureLocalStorageClass(ctx context.Context, lvgNames []string) error {
	desired := k8sadapters.DesiredLocalStorageClass(k8sadapters.LocalStorageClassSpec{
		Name:         r.options.StorageClassName,
		ThinPoolName: r.options.ThinPoolName,
		LVGNames:     lvgNames,
	})
	return r.syncObject(ctx, k8sadapters.NewLocalStorageClass(r.options.StorageClassName), desired)
}

func (r *Reconciler) syncObject(ctx context.Context, current, desired *metav1unstructured.Unstructured) error {
	err := r.client.Get(ctx, types.NamespacedName{Name: desired.GetName()}, current)
	if apierrors.IsNotFound(err) {
		return r.client.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	updated := current.DeepCopy()
	updated.SetLabels(desired.GetLabels())
	updated.Object["spec"] = desired.Object["spec"]
	if equalSpecsAndLabels(current, updated) {
		return nil
	}
	updated.SetResourceVersion(current.GetResourceVersion())
	return r.client.Update(ctx, updated)
}

func equalSpecsAndLabels(left, right *metav1unstructured.Unstructured) bool {
	if !mapsEqual(left.GetLabels(), right.GetLabels()) {
		return false
	}
	leftSpec, _, _ := metav1unstructured.NestedFieldCopy(left.Object, "spec")
	rightSpec, _, _ := metav1unstructured.NestedFieldCopy(right.Object, "spec")
	return reflect.DeepEqual(leftSpec, rightSpec)
}

func mapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}
