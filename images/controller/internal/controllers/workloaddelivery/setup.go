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

package workloaddelivery

import (
	"context"
	"errors"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	deploymentControllerName  = defaultControllerNamePrefix + "-deployment"
	statefulSetControllerName = defaultControllerNamePrefix + "-statefulset"
	daemonSetControllerName   = defaultControllerNamePrefix + "-daemonset"
	cronJobControllerName     = defaultControllerNamePrefix + "-cronjob"
)

var errUnexpectedWorkloadListItem = errors.New("workload list returned a non-object item")

type workloadKind struct {
	kind           string
	controllerName string
	object         func() client.Object
	list           func() client.ObjectList
}

var workloadKinds = []workloadKind{
	{
		kind:           "Deployment",
		controllerName: deploymentControllerName,
		object:         func() client.Object { return &appsv1.Deployment{} },
		list:           func() client.ObjectList { return &appsv1.DeploymentList{} },
	},
	{
		kind:           "StatefulSet",
		controllerName: statefulSetControllerName,
		object:         func() client.Object { return &appsv1.StatefulSet{} },
		list:           func() client.ObjectList { return &appsv1.StatefulSetList{} },
	},
	{
		kind:           "DaemonSet",
		controllerName: daemonSetControllerName,
		object:         func() client.Object { return &appsv1.DaemonSet{} },
		list:           func() client.ObjectList { return &appsv1.DaemonSetList{} },
	},
	{
		kind:           "CronJob",
		controllerName: cronJobControllerName,
		object:         func() client.Object { return &batchv1.CronJob{} },
		list:           func() client.ObjectList { return &batchv1.CronJobList{} },
	},
}

type workloadReconciler struct {
	baseReconciler
	newObject func() client.Object
}

func SetupWithManager(mgr ctrl.Manager, options Options) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}
	options = normalizeOptions(options)
	if !options.Enabled() {
		return nil
	}
	if err := options.Validate(); err != nil {
		return err
	}
	if err := indexWorkloadReferences(context.Background(), mgr.GetFieldIndexer()); err != nil {
		return err
	}

	deliveryService, err := modeldelivery.NewService(mgr.GetClient(), mgr.GetScheme(), options.Service)
	if err != nil {
		return err
	}

	for _, kind := range workloadKinds {
		if err := setupWorkloadController(mgr, kind, baseReconciler{
			client:   mgr.GetClient(),
			reader:   mgr.GetAPIReader(),
			delivery: deliveryService,
			options:  options,
			logger:   controllerLogger(kind.kind),
			recorder: mgr.GetEventRecorderFor(kind.controllerName),
		}); err != nil {
			return err
		}
	}
	return setupAdmission(mgr)
}

func setupWorkloadController(mgr ctrl.Manager, kind workloadKind, base baseReconciler) error {
	controller := ctrl.NewControllerManagedBy(mgr).
		Named(kind.controllerName).
		For(kind.object(), builder.WithPredicates(workloadEventFilter(base.options.Service))).
		Watches(&modelsv1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(base.mapWorkloadsForModel(kind))).
		Watches(&modelsv1alpha1.ClusterModel{}, handler.EnqueueRequestsFromMapFunc(base.mapWorkloadsForClusterModel(kind)))
	if base.options.Service.ManagedCache.Enabled {
		controller = controller.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(base.mapWorkloadsForNodeCacheReadiness(kind)), builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
	}
	return controller.Complete(&workloadReconciler{baseReconciler: base, newObject: kind.object})
}

func (r *workloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	object := r.newObject()
	if err := r.client.Get(ctx, req.NamespacedName, object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcileWorkload(ctx, object)
}
