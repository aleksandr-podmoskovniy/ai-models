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

package uploadsession

import (
	"context"
	"errors"
	"strings"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const TokenSecretAuthorizationHeaderKey = "authorizationHeaderValue"

func (s *Service) ensureTokenSecret(
	ctx context.Context,
	owner client.Object,
	session *uploadsessionstate.Session,
	rawToken string,
) (modelsv1alpha1.UploadTokenSecretReference, error) {
	if owner == nil {
		return modelsv1alpha1.UploadTokenSecretReference{}, errors.New("upload session owner must not be nil")
	}
	if session == nil {
		return modelsv1alpha1.UploadTokenSecretReference{}, errors.New("upload session must not be nil")
	}
	headerValue := buildAuthorizationHeaderValue(rawToken)
	if headerValue == "" {
		return modelsv1alpha1.UploadTokenSecretReference{}, errors.New("upload session token must not be empty")
	}
	name, namespace, err := tokenSecretObjectKey(owner.GetUID(), owner.GetNamespace(), s.options.Runtime.Namespace)
	if err != nil {
		return modelsv1alpha1.UploadTokenSecretReference{}, err
	}
	ownerKind, ownerName, ownerUID, ownerNamespace := tokenSecretOwnerMetadata(owner, session)

	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: resourcenames.OwnerLabels(
				"ai-models-upload-token",
				ownerKind,
				ownerName,
				ownerUID,
				ownerNamespace,
			),
			Annotations: resourcenames.OwnerAnnotations(ownerKind, ownerName, ownerNamespace),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			TokenSecretAuthorizationHeaderKey: []byte(headerValue),
		},
	}
	if !session.ExpiresAt.IsZero() {
		desired.Annotations[uploadsessionstate.ExpiresAtAnnotationKey] = session.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, desired)
	if err != nil {
		return modelsv1alpha1.UploadTokenSecretReference{}, err
	}
	if !created && tokenSecretNeedsUpdate(desired, headerValue, session) {
		desired.Type = corev1.SecretTypeOpaque
		desired.Labels = mergeMaps(desired.Labels, resourcenames.OwnerLabels(
			"ai-models-upload-token",
			ownerKind,
			ownerName,
			ownerUID,
			ownerNamespace,
		))
		desired.Annotations = mergeMaps(desired.Annotations, resourcenames.OwnerAnnotations(ownerKind, ownerName, ownerNamespace))
		if !session.ExpiresAt.IsZero() {
			desired.Annotations[uploadsessionstate.ExpiresAtAnnotationKey] = session.ExpiresAt.Time.UTC().Format(time.RFC3339)
		}
		if desired.Data == nil {
			desired.Data = make(map[string][]byte, 1)
		}
		desired.Data[TokenSecretAuthorizationHeaderKey] = []byte(headerValue)
		if err := s.client.Update(ctx, desired); err != nil {
			return modelsv1alpha1.UploadTokenSecretReference{}, err
		}
	}

	return modelsv1alpha1.UploadTokenSecretReference{
		Namespace: namespace,
		Name:      name,
		Key:       TokenSecretAuthorizationHeaderKey,
	}, nil
}

func (s *Service) tokenFromHandoffSecret(
	ctx context.Context,
	owner client.Object,
	session *uploadsessionstate.Session,
) (string, bool, error) {
	if owner == nil {
		return "", false, errors.New("upload session owner must not be nil")
	}
	name, namespace, err := tokenSecretObjectKey(owner.GetUID(), owner.GetNamespace(), s.options.Runtime.Namespace)
	if err != nil {
		return "", false, err
	}
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	token, ok := tokenFromAuthorizationHeaderValue(string(secret.Data[TokenSecretAuthorizationHeaderKey]))
	if !ok || !tokenMatchesSession(token, session) {
		return "", false, nil
	}
	return token, true, nil
}

func tokenSecretNeedsUpdate(secret *corev1.Secret, headerValue string, session *uploadsessionstate.Session) bool {
	if secret == nil {
		return true
	}
	if secret.Type != corev1.SecretTypeOpaque {
		return true
	}
	if string(secret.Data[TokenSecretAuthorizationHeaderKey]) != headerValue {
		return true
	}
	if session != nil && !session.ExpiresAt.IsZero() {
		want := session.ExpiresAt.Time.UTC().Format(time.RFC3339)
		return strings.TrimSpace(secret.Annotations[uploadsessionstate.ExpiresAtAnnotationKey]) != want
	}
	return false
}

func tokenMatchesSession(rawToken string, session *uploadsessionstate.Session) bool {
	if session == nil {
		return false
	}
	return uploadsessiontoken.Hash(rawToken) == strings.TrimSpace(session.UploadTokenHash)
}

func tokenSecretOwnerMetadata(owner client.Object, session *uploadsessionstate.Session) (string, string, types.UID, string) {
	kind := ""
	name := ""
	uid := types.UID("")
	namespace := ""
	if session != nil {
		kind = strings.TrimSpace(session.OwnerKind)
		name = strings.TrimSpace(session.OwnerName)
		uid = types.UID(strings.TrimSpace(session.OwnerUID))
		namespace = strings.TrimSpace(session.OwnerNamespace)
	}
	if owner != nil {
		if kind == "" {
			kind = strings.TrimSpace(owner.GetObjectKind().GroupVersionKind().Kind)
		}
		if name == "" {
			name = strings.TrimSpace(owner.GetName())
		}
		if uid == "" {
			uid = owner.GetUID()
		}
		if namespace == "" {
			namespace = strings.TrimSpace(owner.GetNamespace())
		}
	}
	return kind, name, uid, namespace
}

func tokenSecretObjectKey(ownerUID types.UID, ownerNamespace, runtimeNamespace string) (string, string, error) {
	name, err := resourcenames.UploadSessionTokenSecretName(ownerUID)
	if err != nil {
		return "", "", err
	}
	namespace := strings.TrimSpace(ownerNamespace)
	if namespace == "" {
		namespace = strings.TrimSpace(runtimeNamespace)
	}
	if namespace == "" {
		return "", "", errors.New("upload token secret namespace must not be empty")
	}
	return name, namespace, nil
}
