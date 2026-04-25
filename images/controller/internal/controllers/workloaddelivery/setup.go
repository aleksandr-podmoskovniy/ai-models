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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	deploymentControllerName  = defaultControllerNamePrefix + "-deployment"
	statefulSetControllerName = defaultControllerNamePrefix + "-statefulset"
	daemonSetControllerName   = defaultControllerNamePrefix + "-daemonset"
	cronJobControllerName     = defaultControllerNamePrefix + "-cronjob"
)

type deploymentReconciler struct{ baseReconciler }
type statefulSetReconciler struct{ baseReconciler }
type daemonSetReconciler struct{ baseReconciler }
type cronJobReconciler struct{ baseReconciler }

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

	if err := setupDeploymentController(mgr, baseReconciler{
		client:   mgr.GetClient(),
		reader:   mgr.GetAPIReader(),
		delivery: deliveryService,
		options:  options,
		logger:   controllerLogger("Deployment"),
		recorder: mgr.GetEventRecorderFor(deploymentControllerName),
	}); err != nil {
		return err
	}
	if err := setupStatefulSetController(mgr, baseReconciler{
		client:   mgr.GetClient(),
		reader:   mgr.GetAPIReader(),
		delivery: deliveryService,
		options:  options,
		logger:   controllerLogger("StatefulSet"),
		recorder: mgr.GetEventRecorderFor(statefulSetControllerName),
	}); err != nil {
		return err
	}
	if err := setupDaemonSetController(mgr, baseReconciler{
		client:   mgr.GetClient(),
		reader:   mgr.GetAPIReader(),
		delivery: deliveryService,
		options:  options,
		logger:   controllerLogger("DaemonSet"),
		recorder: mgr.GetEventRecorderFor(daemonSetControllerName),
	}); err != nil {
		return err
	}
	if err := setupCronJobController(mgr, baseReconciler{
		client:   mgr.GetClient(),
		reader:   mgr.GetAPIReader(),
		delivery: deliveryService,
		options:  options,
		logger:   controllerLogger("CronJob"),
		recorder: mgr.GetEventRecorderFor(cronJobControllerName),
	}); err != nil {
		return err
	}
	return setupAdmission(mgr)
}

func setupDeploymentController(mgr ctrl.Manager, base baseReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(deploymentControllerName).
		For(&appsv1.Deployment{}, builder.WithPredicates(workloadEventFilter(base.options.Service))).
		Watches(&modelsv1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(base.mapDeploymentsForModel)).
		Watches(&modelsv1alpha1.ClusterModel{}, handler.EnqueueRequestsFromMapFunc(base.mapDeploymentsForClusterModel)).
		Complete(&deploymentReconciler{base})
}

func setupStatefulSetController(mgr ctrl.Manager, base baseReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(statefulSetControllerName).
		For(&appsv1.StatefulSet{}, builder.WithPredicates(workloadEventFilter(base.options.Service))).
		Watches(&modelsv1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(base.mapStatefulSetsForModel)).
		Watches(&modelsv1alpha1.ClusterModel{}, handler.EnqueueRequestsFromMapFunc(base.mapStatefulSetsForClusterModel)).
		Complete(&statefulSetReconciler{base})
}

func setupDaemonSetController(mgr ctrl.Manager, base baseReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(daemonSetControllerName).
		For(&appsv1.DaemonSet{}, builder.WithPredicates(workloadEventFilter(base.options.Service))).
		Watches(&modelsv1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(base.mapDaemonSetsForModel)).
		Watches(&modelsv1alpha1.ClusterModel{}, handler.EnqueueRequestsFromMapFunc(base.mapDaemonSetsForClusterModel)).
		Complete(&daemonSetReconciler{base})
}

func setupCronJobController(mgr ctrl.Manager, base baseReconciler) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(cronJobControllerName).
		For(&batchv1.CronJob{}, builder.WithPredicates(workloadEventFilter(base.options.Service))).
		Watches(&modelsv1alpha1.Model{}, handler.EnqueueRequestsFromMapFunc(base.mapCronJobsForModel)).
		Watches(&modelsv1alpha1.ClusterModel{}, handler.EnqueueRequestsFromMapFunc(base.mapCronJobsForClusterModel)).
		Complete(&cronJobReconciler{base})
}

func (r *deploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object appsv1.Deployment
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcileWorkload(ctx, &object)
}

func (r *statefulSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object appsv1.StatefulSet
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcileWorkload(ctx, &object)
}

func (r *daemonSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object appsv1.DaemonSet
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcileWorkload(ctx, &object)
}

func (r *cronJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var object batchv1.CronJob
	if err := r.client.Get(ctx, req.NamespacedName, &object); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	return r.reconcileWorkload(ctx, &object)
}
