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
	"log/slog"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	workloadReferenceIndexField     = "ai.deckhouse.io/workloaddelivery-ref"
	modelReferenceIndexField        = "ai.deckhouse.io/workloaddelivery-model-ref"
	clusterModelReferenceIndexField = "ai.deckhouse.io/workloaddelivery-clustermodel-ref"
	workloadReferenceIndexValue     = "true"
)

func indexWorkloadReferences(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, &appsv1.Deployment{}, workloadReferenceIndexField, workloadReferenceIndex); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.Deployment{}, modelReferenceIndexField, modelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.Deployment{}, clusterModelReferenceIndexField, clusterModelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.StatefulSet{}, workloadReferenceIndexField, workloadReferenceIndex); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.StatefulSet{}, modelReferenceIndexField, modelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.StatefulSet{}, clusterModelReferenceIndexField, clusterModelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.DaemonSet{}, workloadReferenceIndexField, workloadReferenceIndex); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.DaemonSet{}, modelReferenceIndexField, modelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &appsv1.DaemonSet{}, clusterModelReferenceIndexField, clusterModelReferenceIndexValue); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &batchv1.CronJob{}, workloadReferenceIndexField, workloadReferenceIndex); err != nil {
		return err
	}
	if err := indexer.IndexField(ctx, &batchv1.CronJob{}, modelReferenceIndexField, modelReferenceIndexValue); err != nil {
		return err
	}
	return indexer.IndexField(ctx, &batchv1.CronJob{}, clusterModelReferenceIndexField, clusterModelReferenceIndexValue)
}

func workloadReferenceIndex(object client.Object) []string {
	if strings.TrimSpace(object.GetAnnotations()[ModelAnnotation]) == "" &&
		strings.TrimSpace(object.GetAnnotations()[ClusterModelAnnotation]) == "" {
		return nil
	}
	return []string{workloadReferenceIndexValue}
}

func modelReferenceIndexValue(object client.Object) []string {
	name := strings.TrimSpace(object.GetAnnotations()[ModelAnnotation])
	if name == "" {
		return nil
	}
	return []string{name}
}

func clusterModelReferenceIndexValue(object client.Object) []string {
	name := strings.TrimSpace(object.GetAnnotations()[ClusterModelAnnotation])
	if name == "" {
		return nil
	}
	return []string{name}
}

func (r *baseReconciler) mapDeploymentsForModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listDeploymentRequests(ctx, client.InNamespace(object.GetNamespace()), client.MatchingFields{modelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapDeploymentsForClusterModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listDeploymentRequests(ctx, client.MatchingFields{clusterModelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapStatefulSetsForModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listStatefulSetRequests(ctx, client.InNamespace(object.GetNamespace()), client.MatchingFields{modelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapStatefulSetsForClusterModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listStatefulSetRequests(ctx, client.MatchingFields{clusterModelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapDaemonSetsForModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listDaemonSetRequests(ctx, client.InNamespace(object.GetNamespace()), client.MatchingFields{modelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapDaemonSetsForClusterModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listDaemonSetRequests(ctx, client.MatchingFields{clusterModelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapCronJobsForModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listCronJobRequests(ctx, client.InNamespace(object.GetNamespace()), client.MatchingFields{modelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapCronJobsForClusterModel(ctx context.Context, object client.Object) []reconcile.Request {
	return r.listCronJobRequests(ctx, client.MatchingFields{clusterModelReferenceIndexField: object.GetName()})
}

func (r *baseReconciler) mapDeploymentsForNodeCacheReadiness(ctx context.Context, _ client.Object) []reconcile.Request {
	return r.listDeploymentRequests(ctx, client.MatchingFields{workloadReferenceIndexField: workloadReferenceIndexValue})
}

func (r *baseReconciler) mapStatefulSetsForNodeCacheReadiness(ctx context.Context, _ client.Object) []reconcile.Request {
	return r.listStatefulSetRequests(ctx, client.MatchingFields{workloadReferenceIndexField: workloadReferenceIndexValue})
}

func (r *baseReconciler) mapDaemonSetsForNodeCacheReadiness(ctx context.Context, _ client.Object) []reconcile.Request {
	return r.listDaemonSetRequests(ctx, client.MatchingFields{workloadReferenceIndexField: workloadReferenceIndexValue})
}

func (r *baseReconciler) mapCronJobsForNodeCacheReadiness(ctx context.Context, _ client.Object) []reconcile.Request {
	return r.listCronJobRequests(ctx, client.MatchingFields{workloadReferenceIndexField: workloadReferenceIndexValue})
}

func (r *baseReconciler) listDeploymentRequests(ctx context.Context, options ...client.ListOption) []reconcile.Request {
	var list appsv1.DeploymentList
	if err := r.client.List(ctx, &list, options...); err != nil {
		r.logMapListFailure("Deployment", err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for _, item := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: item.Namespace,
			Name:      item.Name,
		}})
	}
	return requests
}

func (r *baseReconciler) listStatefulSetRequests(ctx context.Context, options ...client.ListOption) []reconcile.Request {
	var list appsv1.StatefulSetList
	if err := r.client.List(ctx, &list, options...); err != nil {
		r.logMapListFailure("StatefulSet", err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for _, item := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: item.Namespace,
			Name:      item.Name,
		}})
	}
	return requests
}

func (r *baseReconciler) listDaemonSetRequests(ctx context.Context, options ...client.ListOption) []reconcile.Request {
	var list appsv1.DaemonSetList
	if err := r.client.List(ctx, &list, options...); err != nil {
		r.logMapListFailure("DaemonSet", err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for _, item := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: item.Namespace,
			Name:      item.Name,
		}})
	}
	return requests
}

func (r *baseReconciler) listCronJobRequests(ctx context.Context, options ...client.ListOption) []reconcile.Request {
	var list batchv1.CronJobList
	if err := r.client.List(ctx, &list, options...); err != nil {
		r.logMapListFailure("CronJob", err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for _, item := range list.Items {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: item.Namespace,
			Name:      item.Name,
		}})
	}
	return requests
}

func (r *baseReconciler) logMapListFailure(kind string, err error) {
	logger := r.logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(
		"runtime delivery reference watch list failed",
		slog.String("mappedWorkloadKind", kind),
		slog.Any("error", err),
	)
}
