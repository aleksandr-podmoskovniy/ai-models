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

package nodecacheruntime

import (
	"context"
	"errors"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type Service struct {
	client client.Client
	scheme *runtime.Scheme
}

func NewService(kubeClient client.Client, scheme *runtime.Scheme) (*Service, error) {
	switch {
	case kubeClient == nil:
		return nil, errors.New("node cache runtime service client must not be nil")
	case scheme == nil:
		return nil, errors.New("node cache runtime service scheme must not be nil")
	default:
		return &Service{client: kubeClient, scheme: scheme}, nil
	}
}

func (s *Service) Apply(ctx context.Context, owner *corev1.Node, spec RuntimeSpec) error {
	if s == nil {
		return errors.New("node cache runtime service must not be nil")
	}
	if owner == nil {
		return errors.New("node cache runtime owner must not be nil")
	}

	desiredPVC, err := DesiredPVC(spec)
	if err != nil {
		return err
	}
	if err := s.createOrUpdateOwned(ctx, owner, &corev1.PersistentVolumeClaim{}, desiredPVC); err != nil {
		return err
	}

	desiredPod, err := DesiredPod(spec)
	if err != nil {
		return err
	}
	return s.createOrReplacePod(ctx, owner, desiredPod)
}

func (s *Service) Delete(ctx context.Context, namespace, nodeName string) error {
	podName, err := resourcenames.NodeCacheRuntimePodName(nodeName)
	if err != nil {
		return err
	}
	pvcName, err := resourcenames.NodeCacheRuntimePVCName(nodeName)
	if err != nil {
		return err
	}
	return ownedresource.DeleteAll(ctx, s.client,
		&corev1.Pod{ObjectMeta: objectMeta(namespace, podName)},
		&corev1.PersistentVolumeClaim{ObjectMeta: objectMeta(namespace, pvcName)},
	)
}

func (s *Service) createOrUpdateOwned(ctx context.Context, owner *corev1.Node, object client.Object, desired client.Object) error {
	object.SetName(desired.GetName())
	object.SetNamespace(desired.GetNamespace())

	_, err := controllerutil.CreateOrUpdate(ctx, s.client, object, func() error {
		if err := ownedresource.MaybeSetControllerReference(owner, object, s.scheme); err != nil {
			return err
		}
		switch existing := object.(type) {
		case *corev1.Pod:
			fresh := desired.(*corev1.Pod)
			existing.Labels = fresh.Labels
			existing.Annotations = fresh.Annotations
			existing.Spec = fresh.Spec
		case *corev1.PersistentVolumeClaim:
			fresh := desired.(*corev1.PersistentVolumeClaim)
			existing.Labels = fresh.Labels
			existing.Annotations = fresh.Annotations
			existing.Spec = fresh.Spec
		default:
			return errors.New("unsupported node cache runtime object type")
		}
		return nil
	})
	return err
}

func (s *Service) createOrReplacePod(ctx context.Context, owner *corev1.Node, desired *corev1.Pod) error {
	if err := ownedresource.MaybeSetControllerReference(owner, desired, s.scheme); err != nil {
		return err
	}

	existing := &corev1.Pod{}
	if err := s.client.Get(ctx, client.ObjectKeyFromObject(desired), existing); err != nil {
		if apierrors.IsNotFound(err) {
			return s.client.Create(ctx, desired)
		}
		return err
	}
	if podMatchesDesired(existing, desired) {
		return nil
	}
	if existing.DeletionTimestamp != nil {
		return nil
	}
	return client.IgnoreNotFound(s.client.Delete(ctx, existing))
}

func podMatchesDesired(existing, desired *corev1.Pod) bool {
	return apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(existing.Annotations, desired.Annotations) &&
		apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) &&
		apiequality.Semantic.DeepEqual(existing.OwnerReferences, desired.OwnerReferences)
}

func objectMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: namespace, Name: name}
}
