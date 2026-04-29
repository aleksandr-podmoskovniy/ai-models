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
	"k8s.io/apimachinery/pkg/api/equality"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) keepRayClusterDeliveryPending(
	ctx context.Context,
	object client.Object,
	original client.Object,
	templates []workloadPodTemplate,
) error {
	_, err := r.keepRayClusterDeliveryStopped(ctx, object, original, templates, "", "")
	return err
}

func (r *baseReconciler) blockRayClusterDelivery(
	ctx context.Context,
	object client.Object,
	source client.Object,
	original client.Object,
	templates []workloadPodTemplate,
	cause error,
) (ctrl.Result, error) {
	changed, err := r.keepRayClusterDeliveryStopped(ctx, object, original, templates, deliveryBlockedReasonInvalidSpec, cause.Error())
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		r.recorder.Event(source, "Warning", "ModelDeliveryBlocked", cause.Error())
		r.logger.Info(
			"runtime delivery blocked by generated raycluster spec",
			slog.String("namespace", object.GetNamespace()),
			slog.String("name", object.GetName()),
			slog.String("sourceName", source.GetName()),
			slog.String("reason", deliveryBlockedReasonInvalidSpec),
		)
	}
	return ctrl.Result{}, nil
}

func (r *baseReconciler) removeManagedRayClusterDelivery(
	ctx context.Context,
	object client.Object,
	original client.Object,
	templates []workloadPodTemplate,
) error {
	_, err := r.keepRayClusterDeliveryStopped(ctx, object, original, templates, "", "")
	return err
}

func (r *baseReconciler) keepRayClusterDeliveryStopped(
	ctx context.Context,
	object client.Object,
	original client.Object,
	templates []workloadPodTemplate,
	reason string,
	message string,
) (bool, error) {
	changed, err := r.stopRayClusterTemplates(templates, reason, message)
	if err != nil {
		return false, err
	}
	accessChanged, err := r.cleanupRayClusterProjectedAccess(ctx, object, templates)
	if err != nil {
		return false, err
	}
	if accessChanged {
		changed = true
	}
	if !changed && equality.Semantic.DeepEqual(original, object) {
		return false, nil
	}
	return true, r.client.Patch(ctx, object, client.MergeFrom(original))
}

func (r *baseReconciler) stopRayClusterTemplates(templates []workloadPodTemplate, reason string, message string) (bool, error) {
	changed := false
	for _, ref := range templates {
		if removeManagedTemplateState(ref.Template, r.options.Service) {
			changed = true
		}
		if reason == "" {
			clearDeliveryBlockedState(ref.Template)
		}
		if modeldelivery.EnsureSchedulingGate(ref.Template) {
			changed = true
		}
		if reason != "" {
			setDeliveryBlockedState(ref.Template, reason, message)
		}
		if err := commitTemplate(ref); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func (r *baseReconciler) cleanupRayClusterProjectedAccess(
	ctx context.Context,
	object client.Object,
	templates []workloadPodTemplate,
) (bool, error) {
	if err := ociregistry.DeleteProjectedAccess(ctx, r.client, object.GetNamespace(), object.GetUID()); err != nil {
		return false, err
	}
	runtimeImagePullSecretName, err := resourcenames.RuntimeImagePullSecretName(object.GetUID())
	if err != nil {
		return false, err
	}
	changed, err := removeImagePullSecretFromTemplates(templates, runtimeImagePullSecretName)
	if err != nil {
		return false, err
	}
	if err := ociregistry.DeleteProjectedImagePullSecret(ctx, r.client, object.GetNamespace(), object.GetUID()); err != nil {
		return false, err
	}
	return changed, nil
}

func removeImagePullSecretFromTemplates(templates []workloadPodTemplate, name string) (bool, error) {
	changed := false
	for _, ref := range templates {
		var removed bool
		ref.Template.Spec.ImagePullSecrets, removed = removeImagePullSecretByName(ref.Template.Spec.ImagePullSecrets, name)
		if removed {
			changed = true
		}
		if err := commitTemplate(ref); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func commitTemplate(ref workloadPodTemplate) error {
	if ref.Commit == nil {
		return nil
	}
	return ref.Commit()
}
