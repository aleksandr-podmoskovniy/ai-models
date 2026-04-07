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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
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
		return nil, errors.New("upload session client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("upload session scheme must not be nil")
	}

	options = normalizeOptions(options)
	if err := options.Validate(); err != nil {
		return nil, err
	}

	return &Service{client: client, scheme: scheme, options: options}, nil
}

func (s *Service) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.OperationContext) (*publicationports.UploadSessionHandle, bool, error) {
	if owner == nil {
		return nil, false, errors.New("upload session owner must not be nil")
	}

	plan, err := requestPlan(request)
	if err != nil {
		return nil, false, err
	}

	secret, token, expiresAt, createdSecret, err := s.ensureSecret(ctx, owner, request.Request.Owner.UID)
	if err != nil {
		return nil, false, err
	}
	service, createdService, err := s.ensureService(ctx, owner, request.Request.Owner.UID)
	if err != nil {
		return nil, false, err
	}
	pod, createdPod, err := s.ensurePod(ctx, owner, request, plan, secret.Name)
	if err != nil {
		return nil, false, err
	}

	return publicationports.NewUploadSessionHandle(
		pod.Name,
		pod.Status.Phase,
		workloadpod.TerminationMessage(pod, "upload"),
		buildUploadStatus(pod, service, token, expiresAt),
		func(ctx context.Context) error {
			return s.deleteResources(ctx, pod, service, secret)
		},
	), createdSecret || createdService || createdPod, nil
}

func (s *Service) deleteResources(
	ctx context.Context,
	pod *corev1.Pod,
	service *corev1.Service,
	secret *corev1.Secret,
) error {
	return ownedresource.DeleteAll(ctx, s.client, pod, service, secret)
}
