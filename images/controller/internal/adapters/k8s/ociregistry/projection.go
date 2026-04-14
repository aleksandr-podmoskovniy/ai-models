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

package ociregistry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Projection struct {
	AuthSecretName string
	CASecretName   string
}

func EnsureProjectedAccess(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	namespace string,
	ownerUID types.UID,
	sourceAuthSecretName string,
	sourceCASecretName string,
) (Projection, error) {
	return EnsureProjectedAccessFromSourceNamespace(
		ctx,
		kubeClient,
		scheme,
		owner,
		namespace,
		ownerUID,
		namespace,
		sourceAuthSecretName,
		sourceCASecretName,
	)
}

func EnsureProjectedAccessFromSourceNamespace(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	targetNamespace string,
	ownerUID types.UID,
	sourceNamespace string,
	sourceAuthSecretName string,
	sourceCASecretName string,
) (Projection, error) {
	switch {
	case kubeClient == nil:
		return Projection{}, errors.New("oci registry projection client must not be nil")
	case scheme == nil:
		return Projection{}, errors.New("oci registry projection scheme must not be nil")
	case owner == nil:
		return Projection{}, errors.New("oci registry projection owner must not be nil")
	case strings.TrimSpace(targetNamespace) == "":
		return Projection{}, errors.New("oci registry projection namespace must not be empty")
	case strings.TrimSpace(sourceAuthSecretName) == "":
		return Projection{}, errors.New("oci registry projection source auth secret name must not be empty")
	}
	if strings.TrimSpace(sourceNamespace) == "" {
		sourceNamespace = targetNamespace
	}

	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		return Projection{}, err
	}
	if err := ensureProjectedAuthSecret(ctx, kubeClient, scheme, owner, targetNamespace, authSecretName, sourceNamespace, sourceAuthSecretName); err != nil {
		return Projection{}, err
	}

	projection := Projection{AuthSecretName: authSecretName}
	if strings.TrimSpace(sourceCASecretName) == "" {
		return projection, nil
	}

	caSecretName, err := resourcenames.OCIRegistryCASecretName(ownerUID)
	if err != nil {
		return Projection{}, err
	}
	if err := ensureProjectedCASecret(ctx, kubeClient, scheme, owner, targetNamespace, caSecretName, sourceNamespace, sourceCASecretName); err != nil {
		return Projection{}, err
	}
	projection.CASecretName = caSecretName

	return projection, nil
}

func DeleteProjectedAccess(ctx context.Context, kubeClient client.Client, namespace string, ownerUID types.UID) error {
	switch {
	case kubeClient == nil:
		return errors.New("oci registry projection client must not be nil")
	case strings.TrimSpace(namespace) == "":
		return errors.New("oci registry projection namespace must not be empty")
	}

	authSecretName, err := resourcenames.OCIRegistryAuthSecretName(ownerUID)
	if err != nil {
		return err
	}
	caSecretName, err := resourcenames.OCIRegistryCASecretName(ownerUID)
	if err != nil {
		return err
	}

	return ownedresource.DeleteAll(
		ctx,
		kubeClient,
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: authSecretName, Namespace: namespace}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: caSecretName, Namespace: namespace}},
	)
}

func ensureProjectedAuthSecret(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	targetNamespace string,
	projectedSecretName string,
	sourceNamespace string,
	sourceSecretName string,
) error {
	sourceSecret := &corev1.Secret{}
	sourceKey := client.ObjectKey{Name: sourceSecretName, Namespace: sourceNamespace}
	if err := kubeClient.Get(ctx, sourceKey, sourceSecret); err != nil {
		return err
	}

	projectedData, err := projectedAuthSecretData(sourceSecret)
	if err != nil {
		return err
	}

	projectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      projectedSecretName,
			Namespace: targetNamespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, kubeClient, projectedSecret, func() error {
		projectedSecret.Type = sourceSecret.Type
		projectedSecret.Data = projectedData
		return ownedresource.MaybeSetControllerReference(owner, projectedSecret, scheme)
	})
	return err
}

func ensureProjectedCASecret(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	targetNamespace string,
	projectedSecretName string,
	sourceNamespace string,
	sourceSecretName string,
) error {
	sourceSecret := &corev1.Secret{}
	sourceKey := client.ObjectKey{Name: sourceSecretName, Namespace: sourceNamespace}
	if err := kubeClient.Get(ctx, sourceKey, sourceSecret); err != nil {
		return err
	}

	cert := bytes.TrimSpace(sourceSecret.Data["ca.crt"])
	if len(cert) == 0 {
		return fmt.Errorf("oci registry CA secret %s/%s must contain ca.crt", sourceNamespace, sourceSecretName)
	}

	projectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      projectedSecretName,
			Namespace: targetNamespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, kubeClient, projectedSecret, func() error {
		projectedSecret.Type = corev1.SecretTypeOpaque
		projectedSecret.Data = map[string][]byte{
			"ca.crt": cert,
		}
		return ownedresource.MaybeSetControllerReference(owner, projectedSecret, scheme)
	})
	return err
}

func projectedAuthSecretData(secret *corev1.Secret) (map[string][]byte, error) {
	if secret == nil {
		return nil, errors.New("oci registry auth source secret must not be nil")
	}

	username := bytes.TrimSpace(secret.Data["username"])
	password := bytes.TrimSpace(secret.Data["password"])
	if len(username) == 0 || len(password) == 0 {
		return nil, fmt.Errorf("oci registry auth secret %s/%s must contain username and password", secret.Namespace, secret.Name)
	}

	data := map[string][]byte{
		"username": username,
		"password": password,
	}
	if dockerConfig := bytes.TrimSpace(secret.Data[corev1.DockerConfigJsonKey]); len(dockerConfig) > 0 {
		data[corev1.DockerConfigJsonKey] = dockerConfig
	}

	return data, nil
}
