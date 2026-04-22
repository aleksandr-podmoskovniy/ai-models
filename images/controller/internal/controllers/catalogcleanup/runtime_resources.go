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

package catalogcleanup

import (
	"context"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type publicationRuntimeResources struct {
	sourceWorker  bool
	uploadSession bool
}

func (r publicationRuntimeResources) Present() bool {
	return r.sourceWorker || r.uploadSession
}

func (r *baseReconciler) observePublicationRuntimeResources(
	ctx context.Context,
	ownerUID types.UID,
) (publicationRuntimeResources, error) {
	namespace := r.runtimeNamespace()

	sourceWorkerObjects, err := sourceWorkerRuntimeObjects(ownerUID, namespace)
	if err != nil {
		return publicationRuntimeResources{}, err
	}
	sourceWorkerPresent, err := r.anyExistingObjects(ctx, sourceWorkerObjects...)
	if err != nil {
		return publicationRuntimeResources{}, err
	}

	uploadSessionObjects, err := uploadSessionRuntimeObjects(ownerUID, namespace)
	if err != nil {
		return publicationRuntimeResources{}, err
	}
	uploadSessionPresent, err := r.anyExistingObjects(ctx, uploadSessionObjects...)
	if err != nil {
		return publicationRuntimeResources{}, err
	}

	return publicationRuntimeResources{
		sourceWorker:  sourceWorkerPresent,
		uploadSession: uploadSessionPresent,
	}, nil
}

func (r *baseReconciler) deletePublicationRuntimeResources(ctx context.Context, ownerUID types.UID) error {
	namespace := r.runtimeNamespace()

	sourceWorkerObjects, err := sourceWorkerRuntimeObjects(ownerUID, namespace)
	if err != nil {
		return err
	}
	uploadSessionObjects, err := uploadSessionRuntimeObjects(ownerUID, namespace)
	if err != nil {
		return err
	}
	if err := ociregistry.DeleteProjectedAccess(ctx, r.client, namespace, ownerUID); err != nil {
		return err
	}
	return ownedresource.DeleteAll(ctx, r.client, append(sourceWorkerObjects, uploadSessionObjects...)...)
}

func (r *baseReconciler) anyExistingObjects(ctx context.Context, objects ...client.Object) (bool, error) {
	for _, object := range objects {
		if object == nil {
			continue
		}
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(object), object); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (r *baseReconciler) runtimeNamespace() string {
	if namespace := strings.TrimSpace(r.options.RuntimeNamespace); namespace != "" {
		return namespace
	}
	return r.options.CleanupJob.Namespace
}

func sourceWorkerRuntimeObjects(ownerUID types.UID, namespace string) ([]client.Object, error) {
	podName, err := resourcenames.SourceWorkerPodName(ownerUID)
	if err != nil {
		return nil, err
	}
	authSecretName, err := resourcenames.SourceWorkerAuthSecretName(ownerUID)
	if err != nil {
		return nil, err
	}
	stateSecretName, err := resourcenames.SourceWorkerStateSecretName(ownerUID)
	if err != nil {
		return nil, err
	}
	ociAuthSecretName, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		return nil, err
	}
	ociCASecretName, err := resourcenames.OCIRegistryCASecretName(ownerUID)
	if err != nil {
		return nil, err
	}

	return []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podName, Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: stateSecretName, Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ociAuthSecretName, Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ociCASecretName, Namespace: namespace}},
	}, nil
}

func uploadSessionRuntimeObjects(ownerUID types.UID, namespace string) ([]client.Object, error) {
	secretName, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return nil, err
	}
	return []client.Object{
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace}},
	}, nil
}
