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

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	workloadReferenceIndexField     = "ai.deckhouse.io/workloaddelivery-ref"
	modelReferenceIndexField        = "ai.deckhouse.io/workloaddelivery-model-ref"
	clusterModelReferenceIndexField = "ai.deckhouse.io/workloaddelivery-clustermodel-ref"
	workloadReferenceIndexValue     = "true"
)

func indexWorkloadReferences(ctx context.Context, indexer client.FieldIndexer) error {
	for _, kind := range workloadKinds {
		object := kind.object()
		if err := indexer.IndexField(ctx, object, workloadReferenceIndexField, workloadReferenceIndex); err != nil {
			return err
		}
		if err := indexer.IndexField(ctx, object, modelReferenceIndexField, modelReferenceIndexValue); err != nil {
			return err
		}
		if err := indexer.IndexField(ctx, object, clusterModelReferenceIndexField, clusterModelReferenceIndexValue); err != nil {
			return err
		}
	}
	return nil
}

func workloadReferenceIndex(object client.Object) []string {
	if strings.TrimSpace(object.GetAnnotations()[ModelAnnotation]) == "" &&
		strings.TrimSpace(object.GetAnnotations()[ClusterModelAnnotation]) == "" &&
		strings.TrimSpace(object.GetAnnotations()[ModelRefsAnnotation]) == "" {
		return nil
	}
	return []string{workloadReferenceIndexValue}
}

func modelReferenceIndexValue(object client.Object) []string {
	return referenceNamesByScope(object, ReferenceScopeModel)
}

func clusterModelReferenceIndexValue(object client.Object) []string {
	return referenceNamesByScope(object, ReferenceScopeClusterModel)
}

func referenceNamesByScope(object client.Object, scope ReferenceScope) []string {
	refs, found, err := parseReferences(object.GetAnnotations())
	if err != nil || !found {
		return nil
	}
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.Scope == scope {
			names = append(names, ref.Name)
		}
	}
	return names
}

func (r *baseReconciler) mapWorkloadsForModel(kind workloadKind) handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		return r.listWorkloadRequests(ctx, kind, client.InNamespace(object.GetNamespace()), client.MatchingFields{modelReferenceIndexField: object.GetName()})
	}
}

func (r *baseReconciler) mapWorkloadsForClusterModel(kind workloadKind) handler.MapFunc {
	return func(ctx context.Context, object client.Object) []reconcile.Request {
		return r.listWorkloadRequests(ctx, kind, client.MatchingFields{clusterModelReferenceIndexField: object.GetName()})
	}
}

func (r *baseReconciler) mapWorkloadsForNodeCacheReadiness(kind workloadKind) handler.MapFunc {
	return func(ctx context.Context, _ client.Object) []reconcile.Request {
		return r.listWorkloadRequests(ctx, kind, client.MatchingFields{workloadReferenceIndexField: workloadReferenceIndexValue})
	}
}

func (r *baseReconciler) listWorkloadRequests(ctx context.Context, kind workloadKind, options ...client.ListOption) []reconcile.Request {
	list := kind.list()
	if err := r.client.List(ctx, list, options...); err != nil {
		r.logMapListFailure(kind.kind, err)
		return nil
	}
	items, err := apimeta.ExtractList(list)
	if err != nil {
		r.logMapListFailure(kind.kind, err)
		return nil
	}
	requests := make([]reconcile.Request, 0, len(items))
	for _, item := range items {
		object, ok := item.(client.Object)
		if !ok {
			r.logMapListFailure(kind.kind, errUnexpectedWorkloadListItem)
			return nil
		}
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: object.GetNamespace(),
			Name:      object.GetName(),
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
