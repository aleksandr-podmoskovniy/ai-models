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
	"maps"

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client  client.Client
	service *k8sadapters.Service
	logger  *slog.Logger
	options Options
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if !r.options.Enabled {
		return ctrl.Result{}, nil
	}

	node := &corev1.Node{}
	if err := r.client.Get(ctx, req.NamespacedName, node); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.service.Delete(ctx, r.options.Namespace, req.Name)
		}
		return ctrl.Result{}, err
	}

	if !r.options.MatchesNode(node) {
		if err := r.setRuntimeReadyLabel(ctx, node, false); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.service.Delete(ctx, r.options.Namespace, node.Name)
	}

	if err := r.service.Apply(ctx, node, r.options.runtimeSpec(node)); err != nil {
		return ctrl.Result{}, err
	}
	ready, err := r.service.RuntimeReady(ctx, r.options.Namespace, node.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.setRuntimeReadyLabel(ctx, node, ready); err != nil {
		return ctrl.Result{}, err
	}
	r.logger.Info("node cache runtime reconciled", slog.String("nodeName", node.Name))
	return ctrl.Result{}, nil
}

func (r *Reconciler) setRuntimeReadyLabel(ctx context.Context, node *corev1.Node, ready bool) error {
	if node == nil {
		return nil
	}
	before := node.DeepCopy()
	if ready {
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}
		node.Labels[nodecache.RuntimeReadyNodeLabelKey] = nodecache.RuntimeReadyNodeLabelValue
	} else if len(node.Labels) > 0 {
		delete(node.Labels, nodecache.RuntimeReadyNodeLabelKey)
	}
	if maps.Equal(before.Labels, node.Labels) {
		return nil
	}
	return r.client.Patch(ctx, node, client.MergeFrom(before))
}
