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
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (s *Service) ensureSecret(
	ctx context.Context,
	operation *corev1.ConfigMap,
	ownerUID types.UID,
) (*corev1.Secret, string, metav1.Time, error) {
	secret, token, expiresAt, err := s.buildSecret(ownerUID)
	if err != nil {
		return nil, "", metav1.Time{}, err
	}
	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, operation, secret)
	if err != nil {
		return nil, "", metav1.Time{}, err
	}
	if created {
		return secret, token, expiresAt, nil
	}
	token = strings.TrimSpace(string(secret.Data["token"]))
	expiresAt, err = expiresAtFromSecret(secret)
	if err != nil {
		return nil, "", metav1.Time{}, err
	}
	return secret, token, expiresAt, nil
}

func (s *Service) ensureService(
	ctx context.Context,
	operation *corev1.ConfigMap,
	ownerUID types.UID,
) (*corev1.Service, error) {
	service, err := s.buildService(ownerUID)
	if err != nil {
		return nil, err
	}
	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, operation, service)
	if err != nil {
		return nil, err
	}
	if created {
		return service, nil
	}
	return service, nil
}

func (s *Service) buildSecret(ownerUID types.UID) (*corev1.Secret, string, metav1.Time, error) {
	name, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return nil, "", metav1.Time{}, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, "", metav1.Time{}, err
	}
	expiresAt := metav1.NewTime(time.Now().Add(s.options.TokenTTL).UTC())

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.options.Namespace,
			Annotations: map[string]string{
				"ai-models.deckhouse.io/upload-expires-at": expiresAt.Format(time.RFC3339),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"token": []byte(token),
		},
	}, token, expiresAt, nil
}

func (s *Service) buildService(ownerUID types.UID) (*corev1.Service, error) {
	name, err := resourcenames.UploadSessionServiceName(ownerUID)
	if err != nil {
		return nil, err
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.options.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				serviceLabelKey: name,
			},
			Ports: []corev1.ServicePort{{
				Name:       "upload",
				Protocol:   corev1.ProtocolTCP,
				Port:       uploadPort,
				TargetPort: intstr.FromInt(uploadPort),
			}},
		},
	}, nil
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
