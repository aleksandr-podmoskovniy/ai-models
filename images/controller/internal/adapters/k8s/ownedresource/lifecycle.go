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

package ownedresource

import (
	"context"
	"errors"
	"reflect"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CreateOrGet(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	owner client.Object,
	desired client.Object,
) (bool, error) {
	switch {
	case kubeClient == nil:
		return false, errors.New("owned resource client must not be nil")
	case scheme == nil:
		return false, errors.New("owned resource scheme must not be nil")
	case owner == nil:
		return false, errors.New("owned resource owner must not be nil")
	case desired == nil:
		return false, errors.New("owned resource desired object must not be nil")
	}

	if err := MaybeSetControllerReference(owner, desired, scheme); err != nil {
		return false, err
	}
	if err := kubeClient.Create(ctx, desired); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return false, err
		}
		if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(desired), desired); err != nil {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func MaybeSetControllerReference(owner client.Object, desired client.Object, scheme *runtime.Scheme) error {
	if owner == nil || desired == nil || scheme == nil {
		return nil
	}
	if !canSetControllerReference(owner, desired) {
		return nil
	}
	return controllerutil.SetControllerReference(owner, desired, scheme)
}

func canSetControllerReference(owner client.Object, desired client.Object) bool {
	ownerNamespace := strings.TrimSpace(owner.GetNamespace())
	desiredNamespace := strings.TrimSpace(desired.GetNamespace())
	return ownerNamespace == "" || ownerNamespace == desiredNamespace
}

func DeleteAll(ctx context.Context, kubeClient client.Client, objects ...client.Object) error {
	if kubeClient == nil {
		return errors.New("owned resource client must not be nil")
	}

	for _, object := range objects {
		if isNilObject(object) {
			continue
		}
		if err := client.IgnoreNotFound(kubeClient.Delete(ctx, object)); err != nil {
			return err
		}
	}

	return nil
}

func isNilObject(object client.Object) bool {
	if object == nil {
		return true
	}
	value := reflect.ValueOf(object)
	return value.Kind() == reflect.Ptr && value.IsNil()
}
