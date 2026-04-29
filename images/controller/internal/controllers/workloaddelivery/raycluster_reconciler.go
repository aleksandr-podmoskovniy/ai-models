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
	"log/slog"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errRayServiceOwnerNotFound = errors.New("raycluster has no RayService owner")

type rayDeliverySource struct {
	object     client.Object
	references []Reference
	found      bool
}

func (r *baseReconciler) reconcileRayCluster(ctx context.Context, object client.Object) (ctrl.Result, error) {
	original := object.DeepCopyObject().(client.Object)

	templates, err := rayClusterPodTemplates(object)
	if err != nil {
		return ctrl.Result{}, err
	}
	managed := hasManagedTemplateStateInAny(templates, r.options.Service)

	source, err := r.rayDeliverySource(ctx, object)
	if err != nil {
		if errors.Is(err, errRayServiceOwnerNotFound) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	resolution, proceed, err := r.prepareRayClusterDeliveryResolution(ctx, object, original, templates, source, managed)
	if err != nil || !proceed {
		return ctrl.Result{}, err
	}

	var firstResult modeldelivery.ApplyResult
	for index, ref := range templates {
		result, err := r.delivery.ApplyToPodTemplate(ctx, object, modeldelivery.ApplyRequest{
			Artifact:        resolution.Artifact,
			ArtifactFamily:  resolution.Family,
			Bindings:        resolution.modelDeliveryBindings(usesModelRefsAnnotation(source.object.GetAnnotations())),
			TargetNamespace: object.GetNamespace(),
			Topology:        ref.Hints,
		}, ref.Template)
		if err != nil {
			if modeldelivery.IsWorkloadContractError(err) {
				return r.blockRayClusterDelivery(ctx, object, source.object, original, templates, err)
			}
			r.recorder.Event(source.object, "Warning", "ModelDeliveryFailed", err.Error())
			return ctrl.Result{}, err
		}
		clearDeliveryBlockedState(ref.Template)
		if ref.Commit != nil {
			if err := ref.Commit(); err != nil {
				return ctrl.Result{}, err
			}
		}
		if index == 0 {
			firstResult = result
		}
	}

	if equality.Semantic.DeepEqual(original, object) {
		return ctrl.Result{}, nil
	}
	if err := r.client.Patch(ctx, object, client.MergeFrom(original)); err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info(
		"runtime delivery applied to generated raycluster",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("sourceKind", source.object.GetObjectKind().GroupVersionKind().Kind),
		slog.String("sourceName", source.object.GetName()),
		slog.String("artifactDigest", resolution.Artifact.Digest),
		slog.Int("modelCount", resolution.modelCount()),
		slog.Int("podTemplateCount", len(templates)),
		slog.String("modelPath", firstResult.ModelPath),
		slog.String("topologyKind", string(firstResult.TopologyKind)),
		slog.String("deliveryMode", string(firstResult.DeliveryMode)),
		slog.String("deliveryReason", string(firstResult.DeliveryReason)),
	)
	r.recorder.Eventf(
		source.object,
		"Normal",
		"ModelDeliveryApplied",
		"Applied runtime delivery for %d model(s) to generated RayCluster %s, primary digest %s with mode %s (%s)",
		resolution.modelCount(),
		object.GetName(),
		resolution.Artifact.Digest,
		firstResult.DeliveryMode,
		firstResult.DeliveryReason,
	)
	return ctrl.Result{}, nil
}

func (r *baseReconciler) prepareRayClusterDeliveryResolution(
	ctx context.Context,
	object client.Object,
	original client.Object,
	templates []workloadPodTemplate,
	source rayDeliverySource,
	managed bool,
) (Resolution, bool, error) {
	if !source.found {
		if !managed {
			return Resolution{}, false, nil
		}
		if err := r.removeManagedRayClusterDelivery(ctx, object, original, templates); err != nil {
			return Resolution{}, false, err
		}
		r.recorder.Event(source.object, "Normal", "ModelDeliveryRemoved", "Removed managed runtime delivery mutation from generated RayCluster")
		return Resolution{}, false, nil
	}

	resolution, err := r.resolveReferences(ctx, source.object.GetNamespace(), source.references)
	if err != nil {
		return Resolution{}, false, err
	}
	if resolution.Ready {
		return resolution, true, nil
	}
	if err := r.keepRayClusterDeliveryPending(ctx, object, original, templates); err != nil {
		return Resolution{}, false, err
	}
	r.recorder.Event(source.object, "Normal", "ModelDeliveryPending", resolution.Message)
	return Resolution{}, false, nil
}

func (r *baseReconciler) rayDeliverySource(ctx context.Context, rayCluster client.Object) (rayDeliverySource, error) {
	references, found, err := parseReferences(rayCluster.GetAnnotations())
	if err != nil || found {
		return rayDeliverySource{object: rayCluster, references: references, found: found}, err
	}

	source, found, err := r.rayServiceOwnerObject(ctx, rayCluster)
	if err != nil {
		return rayDeliverySource{}, err
	}
	if !found {
		return rayDeliverySource{}, errRayServiceOwnerNotFound
	}
	references, found, err = parseReferences(source.GetAnnotations())
	return rayDeliverySource{object: source, references: references, found: found}, err
}

func (r *baseReconciler) rayServiceOwnerObject(ctx context.Context, rayCluster client.Object) (client.Object, bool, error) {
	ref, found := rayServiceOwner(rayCluster)
	if !found {
		return nil, false, nil
	}
	reader := r.reader
	if reader == nil {
		reader = r.client
	}
	source := newRayServiceObject()
	if err := reader.Get(ctx, client.ObjectKey{Namespace: rayCluster.GetNamespace(), Name: ref.Name}, source); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return source, true, nil
}
