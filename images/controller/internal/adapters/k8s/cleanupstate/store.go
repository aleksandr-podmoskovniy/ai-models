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

package cleanupstate

import (
	"context"
	"errors"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AppName                  = "ai-models-cleanup-state"
	DataKey                  = "cleanupHandle"
	CompletedAtAnnotationKey = "ai.deckhouse.io/cleanup-completed-at"
)

type Store struct {
	client    client.Client
	namespace string
}

func New(client client.Client, namespace string) (*Store, error) {
	if client == nil {
		return nil, errors.New("cleanup state client must not be nil")
	}
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, errors.New("cleanup state namespace must not be empty")
	}
	return &Store{client: client, namespace: namespace}, nil
}

func (s *Store) Get(ctx context.Context, owner client.Object) (cleanuphandle.Handle, bool, error) {
	secret, found, err := s.getSecret(ctx, owner)
	if err != nil || !found {
		return cleanuphandle.Handle{}, found, err
	}

	raw := strings.TrimSpace(string(secret.Data[DataKey]))
	if raw == "" {
		return cleanuphandle.Handle{}, false, nil
	}
	handle, err := cleanuphandle.Decode(raw)
	if err != nil {
		return cleanuphandle.Handle{}, false, err
	}
	return handle, true, nil
}

func (s *Store) Exists(ctx context.Context, owner client.Object) (bool, error) {
	_, found, err := s.Get(ctx, owner)
	return found, err
}

func (s *Store) Completed(ctx context.Context, owner client.Object) (bool, error) {
	secret, found, err := s.getSecret(ctx, owner)
	if err != nil || !found {
		return false, err
	}
	return strings.TrimSpace(secret.Annotations[CompletedAtAnnotationKey]) != "", nil
}

func (s *Store) MarkCompleted(ctx context.Context, owner client.Object) error {
	secret, found, err := s.getSecret(ctx, owner)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("cleanup state secret not found")
	}
	if strings.TrimSpace(secret.Annotations[CompletedAtAnnotationKey]) != "" {
		return nil
	}
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[CompletedAtAnnotationKey] = time.Now().UTC().Format(time.RFC3339Nano)
	return s.client.Update(ctx, secret)
}

func (s *Store) UploadStage(ctx context.Context, owner client.Object) (*cleanuphandle.UploadStagingHandle, error) {
	handle, found, err := s.Get(ctx, owner)
	if err != nil || !found || handle.Kind != cleanuphandle.KindUploadStaging {
		return nil, err
	}
	return handle.UploadStaging, nil
}

func (s *Store) Ensure(ctx context.Context, owner client.Object, handle cleanuphandle.Handle) (bool, error) {
	if err := handle.Validate(); err != nil {
		return false, err
	}
	desired, err := s.secretFor(owner, handle)
	if err != nil {
		return false, err
	}

	current := &corev1.Secret{}
	key := client.ObjectKey{Namespace: desired.Namespace, Name: desired.Name}
	if err := s.client.Get(ctx, key, current); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		return true, s.client.Create(ctx, desired)
	}

	if sameSecretState(current, desired) {
		return false, nil
	}
	current.Labels = desired.Labels
	current.Annotations = desired.Annotations
	current.Type = desired.Type
	current.Data = desired.Data
	return true, s.client.Update(ctx, current)
}

func (s *Store) Delete(ctx context.Context, owner client.Object) error {
	name, err := s.nameFor(owner)
	if err != nil {
		return err
	}
	err = s.client.Delete(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: s.namespace},
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (s *Store) getSecret(ctx context.Context, owner client.Object) (*corev1.Secret, bool, error) {
	name, err := s.nameFor(owner)
	if err != nil {
		return nil, false, err
	}
	secret := &corev1.Secret{}
	err = s.client.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, secret)
	if apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return secret, true, nil
}

func (s *Store) secretFor(owner client.Object, handle cleanuphandle.Handle) (*corev1.Secret, error) {
	if owner == nil {
		return nil, errors.New("cleanup state owner must not be nil")
	}
	encoded, err := cleanuphandle.Encode(handle)
	if err != nil {
		return nil, err
	}
	name, err := s.nameFor(owner)
	if err != nil {
		return nil, err
	}
	kind := ownerKind(owner)
	namespace := strings.TrimSpace(owner.GetNamespace())
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   s.namespace,
			Labels:      resourcenames.OwnerLabels(AppName, kind, owner.GetName(), owner.GetUID(), namespace),
			Annotations: resourcenames.OwnerAnnotations(kind, owner.GetName(), namespace),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{DataKey: []byte(encoded)},
	}, nil
}

func (s *Store) nameFor(owner client.Object) (string, error) {
	if owner == nil {
		return "", errors.New("cleanup state owner must not be nil")
	}
	return resourcenames.CleanupHandleSecretName(owner.GetUID())
}

func ownerKind(owner client.Object) string {
	switch owner.(type) {
	case *modelsv1alpha1.Model:
		return modelsv1alpha1.ModelKind
	case *modelsv1alpha1.ClusterModel:
		return modelsv1alpha1.ClusterModelKind
	default:
		return strings.TrimSpace(owner.GetObjectKind().GroupVersionKind().Kind)
	}
}

func sameSecretState(current, desired *corev1.Secret) bool {
	return current.Type == desired.Type &&
		apiequality.Semantic.DeepEqual(current.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(current.Annotations, desired.Annotations) &&
		apiequality.Semantic.DeepEqual(current.Data, desired.Data)
}
