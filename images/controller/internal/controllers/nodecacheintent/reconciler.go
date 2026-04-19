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

	k8sadapters "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheintent"
	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
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
			return ctrl.Result{}, r.service.DeleteConfigMap(ctx, r.options.Namespace, req.Name)
		}
		return ctrl.Result{}, err
	}

	intents, err := r.desiredNodeIntents(ctx, node.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(intents) == 0 {
		return ctrl.Result{}, r.service.DeleteConfigMap(ctx, r.options.Namespace, node.Name)
	}
	if err := r.service.ApplyConfigMap(ctx, r.options.Namespace, node.Name, intents); err != nil {
		return ctrl.Result{}, err
	}
	r.logger.Info("node cache intent reconciled", slog.String("nodeName", node.Name), slog.Int("artifactCount", len(intents)))
	return ctrl.Result{}, nil
}

func (r *Reconciler) desiredNodeIntents(ctx context.Context, nodeName string) ([]intentcontract.ArtifactIntent, error) {
	podList := &corev1.PodList{}
	if err := r.client.List(ctx, podList, client.MatchingFields{podNodeNameIndexField: nodeName}); err != nil {
		return nil, err
	}

	intents := make([]intentcontract.ArtifactIntent, 0, len(podList.Items))
	for index := range podList.Items {
		pod := &podList.Items[index]
		if !k8sadapters.IsActiveScheduledPod(pod) {
			continue
		}
		intent, found, err := k8sadapters.IntentFromPod(pod)
		if err != nil {
			return nil, err
		}
		if found {
			intents = append(intents, intent)
		}
	}
	return intentcontract.NormalizeIntents(intents)
}
