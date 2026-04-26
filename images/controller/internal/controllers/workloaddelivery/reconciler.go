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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type baseReconciler struct {
	client   client.Client
	reader   client.Reader
	delivery *modeldelivery.Service
	options  Options
	logger   *slog.Logger
	recorder record.EventRecorder
}

type deliveryPatchResult struct {
	currentState deliverySignalState
	desiredState deliverySignalState
	patched      bool
}

func (r *baseReconciler) reconcileWorkload(ctx context.Context, object client.Object) (ctrl.Result, error) {
	original := object.DeepCopyObject().(client.Object)

	template, hints, err := podTemplateAndHints(object)
	if err != nil {
		return ctrl.Result{}, err
	}
	managed := hasManagedTemplateState(template, r.options.Service)

	resolution, proceed, err := r.prepareDeliveryResolution(ctx, object, original, template, managed)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !proceed {
		return ctrl.Result{}, nil
	}

	result, err := r.delivery.ApplyToPodTemplate(ctx, object, modeldelivery.ApplyRequest{
		Artifact:        resolution.Artifact,
		ArtifactFamily:  resolution.Family,
		Bindings:        resolution.modelDeliveryBindings(usesModelRefsAnnotation(object.GetAnnotations())),
		TargetNamespace: object.GetNamespace(),
		Topology:        hints,
	}, template)
	if err != nil {
		r.recorder.Event(object, "Warning", "ModelDeliveryFailed", err.Error())
		return ctrl.Result{}, err
	}

	patchResult, err := r.patchAppliedWorkload(ctx, object, original, template)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !patchResult.patched || patchResult.currentState == patchResult.desiredState {
		return ctrl.Result{}, nil
	}

	message := "runtime delivery changed"
	if patchResult.currentState.empty() {
		message = "runtime delivery applied"
	}
	r.logger.Info(
		message,
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("digest", resolution.Artifact.Digest),
		slog.Int("modelCount", resolution.modelCount()),
		slog.String("previousDigest", patchResult.currentState.Digest),
		slog.String("modelPath", result.ModelPath),
		slog.String("previousModelPath", patchResult.currentState.ModelPath),
		slog.String("topologyKind", string(result.TopologyKind)),
		slog.String("deliveryMode", string(result.DeliveryMode)),
		slog.String("previousDeliveryMode", patchResult.currentState.DeliveryMode),
		slog.String("deliveryReason", string(result.DeliveryReason)),
		slog.String("previousDeliveryReason", patchResult.currentState.DeliveryReason),
	)
	r.recorder.Eventf(
		object,
		"Normal",
		"ModelDeliveryApplied",
		"Applied runtime delivery for %d model(s), primary digest %s with mode %s (%s)",
		resolution.modelCount(),
		resolution.Artifact.Digest,
		result.DeliveryMode,
		result.DeliveryReason,
	)

	return ctrl.Result{}, nil
}

func (r *baseReconciler) currentWorkload(ctx context.Context, object client.Object) (client.Object, error) {
	reader := r.reader
	if reader == nil {
		reader = r.client
	}

	current, err := newWorkloadObjectLike(object)
	if err != nil {
		return nil, err
	}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(object), current); err != nil {
		return nil, err
	}
	return current, nil
}

func (r *baseReconciler) prepareDeliveryResolution(
	ctx context.Context,
	object client.Object,
	original client.Object,
	template *corev1.PodTemplateSpec,
	managed bool,
) (Resolution, bool, error) {
	references, found, err := parseReferences(object.GetAnnotations())
	if err != nil {
		if managed {
			if err := r.removeManagedDelivery(ctx, object, original, template); err != nil {
				return Resolution{}, false, err
			}
		}
		r.recorder.Event(object, "Warning", "InvalidModelReference", err.Error())
		return Resolution{}, false, nil
	}
	if !found {
		if !managed {
			return Resolution{}, false, nil
		}
		if err := r.removeManagedDelivery(ctx, object, original, template); err != nil {
			return Resolution{}, false, err
		}
		r.recorder.Event(object, "Normal", "ModelDeliveryRemoved", "Removed managed runtime delivery mutation")
		return Resolution{}, false, nil
	}

	resolution, err := r.resolveReferences(ctx, object.GetNamespace(), references)
	if err != nil {
		return Resolution{}, false, err
	}
	if resolution.Ready {
		return resolution, true, nil
	}

	if err := r.keepManagedDeliveryPending(ctx, object, original, template); err != nil {
		return Resolution{}, false, err
	}
	r.recorder.Event(object, "Normal", "ModelDeliveryPending", resolution.Message)
	return Resolution{}, false, nil
}

func (r *baseReconciler) patchAppliedWorkload(
	ctx context.Context,
	object client.Object,
	original client.Object,
	template *corev1.PodTemplateSpec,
) (deliveryPatchResult, error) {
	if equality.Semantic.DeepEqual(original, object) {
		return deliveryPatchResult{}, nil
	}

	current, err := r.currentWorkload(ctx, object)
	if err != nil {
		return deliveryPatchResult{}, client.IgnoreNotFound(err)
	}
	currentTemplate, _, err := podTemplateAndHints(current)
	if err != nil {
		return deliveryPatchResult{}, err
	}
	if equality.Semantic.DeepEqual(currentTemplate, template) {
		return deliveryPatchResult{}, nil
	}

	result := deliveryPatchResult{
		currentState: deliverySignalStateFromTemplate(currentTemplate),
		desiredState: deliverySignalStateFromTemplate(template),
	}
	if err := r.client.Patch(ctx, object, client.MergeFrom(original)); err != nil {
		return deliveryPatchResult{}, err
	}
	result.patched = true
	return result, nil
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
	runtimeImagePullSecretName, err := resourcenames.RuntimeImagePullSecretName(object.GetUID())
	if err != nil {
		return err
	}
	var removed bool
	template.Spec.ImagePullSecrets, removed = removeImagePullSecretByName(template.Spec.ImagePullSecrets, runtimeImagePullSecretName)
	if removed {
		changed = true
	}
	if err := ociregistry.DeleteProjectedImagePullSecret(ctx, r.client, object.GetNamespace(), object.GetUID()); err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return r.client.Patch(ctx, object, client.MergeFrom(original))
}
