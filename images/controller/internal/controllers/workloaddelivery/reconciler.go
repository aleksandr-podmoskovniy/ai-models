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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type baseReconciler struct {
	client   client.Client
	delivery *modeldelivery.Service
	options  Options
	logger   *slog.Logger
	recorder record.EventRecorder
}

func (r *baseReconciler) reconcileWorkload(ctx context.Context, object client.Object) (ctrl.Result, error) {
	original := object.DeepCopyObject().(client.Object)

	template, hints, err := podTemplateAndHints(object)
	if err != nil {
		return ctrl.Result{}, err
	}
	managed := hasManagedTemplateState(template, r.options.Service)

	reference, found, err := parseReference(object.GetAnnotations())
	if err != nil {
		if managed {
			if err := r.removeManagedDelivery(ctx, object, original, template); err != nil {
				return ctrl.Result{}, err
			}
		}
		r.recorder.Event(object, "Warning", "InvalidModelReference", err.Error())
		return ctrl.Result{}, nil
	}
	if !found {
		if !managed {
			return ctrl.Result{}, nil
		}
		if err := r.removeManagedDelivery(ctx, object, original, template); err != nil {
			return ctrl.Result{}, err
		}
		r.recorder.Event(object, "Normal", "ModelDeliveryRemoved", "Removed managed runtime delivery mutation")
		return ctrl.Result{}, nil
	}

	resolution, err := r.resolveReference(ctx, object.GetNamespace(), reference)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !resolution.Ready {
		if err := r.removeManagedDelivery(ctx, object, original, template); err != nil {
			return ctrl.Result{}, err
		}
		r.recorder.Event(object, "Normal", "ModelDeliveryPending", resolution.Message)
		return ctrl.Result{}, nil
	}

	result, err := r.delivery.ApplyToPodTemplate(ctx, object, modeldelivery.ApplyRequest{
		Artifact:        resolution.Artifact,
		ArtifactFamily:  resolution.Family,
		TargetNamespace: object.GetNamespace(),
		Topology:        hints,
	}, template)
	if err != nil {
		r.recorder.Event(object, "Warning", "ModelDeliveryFailed", err.Error())
		return ctrl.Result{}, err
	}

	if equality.Semantic.DeepEqual(original, object) {
		return ctrl.Result{}, nil
	}
	if err := r.client.Patch(ctx, object, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}
	r.logger.Info(
		"runtime delivery applied",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("digest", resolution.Artifact.Digest),
		slog.String("modelPath", result.ModelPath),
		slog.String("topologyKind", string(result.TopologyKind)),
	)
	r.recorder.Eventf(object, "Normal", "ModelDeliveryApplied", "Applied runtime delivery for digest %s", resolution.Artifact.Digest)

	return ctrl.Result{}, nil
}

func (r *baseReconciler) removeManagedDelivery(
	ctx context.Context,
	object client.Object,
	original client.Object,
	template *corev1.PodTemplateSpec,
) error {
	changed := removeManagedTemplateState(template, r.options.Service)
	if err := ociregistry.DeleteProjectedAccess(ctx, r.client, object.GetNamespace(), object.GetUID()); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return r.client.Patch(ctx, object, client.MergeFrom(original))
}
