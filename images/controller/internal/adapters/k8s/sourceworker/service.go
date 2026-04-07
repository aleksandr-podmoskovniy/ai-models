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
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client  client.Client
	scheme  *runtime.Scheme
	options Options
}

func NewService(client client.Client, scheme *runtime.Scheme, options Options) (*Service, error) {
	if client == nil {
		return nil, errors.New("source worker service client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("source worker service scheme must not be nil")
	}
	options = workloadpod.NormalizeRuntimeOptions(options)
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	return &Service{
		client:  client,
		scheme:  scheme,
		options: options,
	}, nil
}

func (s *Service) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.OperationContext) (*publicationports.SourceWorkerHandle, bool, error) {
	if s == nil {
		return nil, false, errors.New("source worker service must not be nil")
	}
	if owner == nil {
		return nil, false, errors.New("source worker owner must not be nil")
	}

	plan, err := sourcePlan(request)
	if err != nil {
		return nil, false, err
	}

	projectedAuthSecretName, err := s.ensureProjectedAuthSecret(ctx, owner, request.Request.Owner, plan)
	if err != nil {
		return nil, false, err
	}

	pod, err := buildWithPlan(request, plan, s.options, projectedAuthSecretName)
	if err != nil {
		return nil, false, err
	}

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, pod)
	if err != nil {
		return nil, false, err
	}

	return s.handleFromPod(pod), created, nil
}

func (s *Service) handleFromPod(pod *corev1.Pod) *publicationports.SourceWorkerHandle {
	return publicationports.NewSourceWorkerHandle(
		pod.Name,
		pod.Status.Phase,
		workloadpod.TerminationMessage(pod, "publish"),
		func(ctx context.Context) error {
			return s.deleteResources(ctx, pod)
		},
	)
}

func (s *Service) deleteResources(ctx context.Context, pod *corev1.Pod) error {
	if s == nil || pod == nil {
		return nil
	}
	secret, err := s.projectedAuthSecretForPod(pod)
	if err != nil {
		return err
	}
	return ownedresource.DeleteAll(ctx, s.client, secret, pod)
}

func (s *Service) projectedAuthSecretForPod(pod *corev1.Pod) (*corev1.Secret, error) {
	ownerUID, ok := resourcenames.OwnerUIDFromLabels(pod.Labels)
	if !ok {
		return nil, nil
	}

	secretName, err := resourcenames.SourceWorkerAuthSecretName(ownerUID)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: s.options.Namespace,
		},
	}, nil
}
