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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) removeManagedDelivery(
	ctx context.Context,
	object client.Object,
	original client.Object,
	template *corev1.PodTemplateSpec,
) error {
	changed := removeManagedTemplateState(template, r.options.Service)
	finalizerRemoved := removeDeliveryFinalizer(object)
	if err := deleteLegacyProjectedAccess(ctx, r.client, object); err != nil {
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
	if !changed && !finalizerRemoved {
		return nil
	}
	return r.client.Patch(ctx, object, client.MergeFrom(original))
}

func deleteLegacyProjectedAccess(ctx context.Context, kubeClient client.Client, object client.Object) error {
	if object == nil {
		return nil
	}
	if err := ociregistry.DeleteProjectedAccess(ctx, kubeClient, object.GetNamespace(), object.GetUID()); err != nil {
		return err
	}
	return ociregistry.DeleteProjectedImagePullSecret(ctx, kubeClient, object.GetNamespace(), object.GetUID())
}
