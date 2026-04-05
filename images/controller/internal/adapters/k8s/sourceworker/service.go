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

package sourceworker

import (
	"context"
	"errors"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client  client.Client
	scheme  *runtime.Scheme
	options Options
}

func NewService(client client.Client, scheme *runtime.Scheme, options Options) (*Service, error) {
	if client == nil {
		return nil, errors.New("source publish pod service client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("source publish pod service scheme must not be nil")
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}

	return &Service{
		client:  client,
		scheme:  scheme,
		options: options,
	}, nil
}

func NewRuntime(client client.Client, scheme *runtime.Scheme, options Options) (publicationports.SourceWorkerRuntime, error) {
	return NewService(client, scheme, options)
}

func (s *Service) Get(ctx context.Context, ownerUID types.UID) (*publicationports.SourceWorkerHandle, error) {
	pod, err := s.getPod(ctx, ownerUID)
	if err != nil {
		return nil, err
	}

	return sourceWorkerHandleFromPod(s, pod), nil
}

func (s *Service) getPod(ctx context.Context, ownerUID types.UID) (*corev1.Pod, error) {
	if s == nil {
		return nil, errors.New("source publish pod service must not be nil")
	}

	name, err := resourcenames.SourceWorkerPodName(ownerUID)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: s.options.Namespace}, pod); err != nil {
		return nil, err
	}

	return pod, nil
}

func (s *Service) GetOrCreate(ctx context.Context, operation *corev1.ConfigMap, request publicationports.OperationContext) (*publicationports.SourceWorkerHandle, bool, error) {
	pod, created, err := s.getOrCreatePod(ctx, operation, request)
	if err != nil {
		return nil, false, err
	}

	return sourceWorkerHandleFromPod(s, pod), created, nil
}

func (s *Service) getOrCreatePod(ctx context.Context, operation *corev1.ConfigMap, request publicationports.OperationContext) (*corev1.Pod, bool, error) {
	if s == nil {
		return nil, false, errors.New("source publish pod service must not be nil")
	}
	if operation == nil {
		return nil, false, errors.New("source publish pod operation configmap must not be nil")
	}
	if operation.Namespace != s.options.Namespace {
		return nil, false, errors.New("source publish pod operation namespace must match worker namespace")
	}

	pod, err := s.getPod(ctx, request.Request.Owner.UID)
	if err == nil {
		return pod, false, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, false, err
	}

	request.OperationName = operation.Name
	request.OperationNamespace = operation.Namespace

	plan, err := sourcePlan(request)
	if err != nil {
		return nil, false, err
	}

	projectedAuthSecretName, err := s.ensureProjectedAuthSecret(ctx, operation, request.Request.Owner, plan)
	if err != nil {
		return nil, false, err
	}

	pod, err = buildWithPlan(request, plan, s.options, projectedAuthSecretName)
	if err != nil {
		return nil, false, err
	}

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, operation, pod)
	if err != nil {
		return nil, false, err
	}

	return pod, created, nil
}

func (s *Service) deletePod(ctx context.Context, pod *corev1.Pod) error {
	if s == nil || pod == nil {
		return nil
	}
	if err := s.deleteProjectedAuthSecret(ctx, pod); err != nil {
		return err
	}
	return client.IgnoreNotFound(s.client.Delete(ctx, pod))
}

func (s *Service) deleteProjectedAuthSecret(ctx context.Context, pod *corev1.Pod) error {
	ownerUID, ok := resourcenames.OwnerUIDFromLabels(pod.Labels)
	if !ok {
		return nil
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(ownerUID)
	if err != nil {
		return err
	}

	secret := &corev1.Secret{}
	secret.Name = secretName
	secret.Namespace = s.options.Namespace
	return client.IgnoreNotFound(s.client.Delete(ctx, secret))
}

func sourceWorkerHandleFromPod(service *Service, pod *corev1.Pod) *publicationports.SourceWorkerHandle {
	if pod == nil {
		return nil
	}

	return publicationports.NewSourceWorkerHandle(
		pod.Name,
		pod.Status.Phase,
		func(ctx context.Context) error {
			return service.deletePod(ctx, pod)
		},
	)
}
