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

	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publication"
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

func NewRuntime(client client.Client, scheme *runtime.Scheme, options Options) (publicationports.UploadSessionRuntime, error) {
	return NewService(client, scheme, options)
}

func (s *Service) Get(ctx context.Context, ownerUID types.UID) (*publicationports.UploadSessionHandle, error) {
	session, err := s.getSession(ctx, ownerUID)
	if err != nil {
		return nil, err
	}

	return uploadSessionHandleFromSession(s, session), nil
}

func (s *Service) getSession(ctx context.Context, ownerUID types.UID) (*Session, error) {
	podName, err := resourcenames.UploadSessionPodName(ownerUID)
	if err != nil {
		return nil, err
	}
	serviceName, err := resourcenames.UploadSessionServiceName(ownerUID)
	if err != nil {
		return nil, err
	}
	secretName, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return nil, err
	}

	pod := &corev1.Pod{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: s.options.Namespace}, pod); err != nil {
		return nil, err
	}
	service := &corev1.Service{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: s.options.Namespace}, service); err != nil {
		return nil, err
	}
	secret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: secretName, Namespace: s.options.Namespace}, secret); err != nil {
		return nil, err
	}

	return sessionFromResources(pod, service, secret)
}

func (s *Service) GetOrCreate(ctx context.Context, operation *corev1.ConfigMap, request publicationports.OperationContext) (*publicationports.UploadSessionHandle, bool, error) {
	session, created, err := s.getOrCreateSession(ctx, operation, request)
	if err != nil {
		return nil, false, err
	}

	return uploadSessionHandleFromSession(s, session), created, nil
}

func (s *Service) getOrCreateSession(ctx context.Context, operation *corev1.ConfigMap, request publicationports.OperationContext) (*Session, bool, error) {
	if operation == nil {
		return nil, false, errors.New("upload session operation configmap must not be nil")
	}
	if operation.Namespace != s.options.Namespace {
		return nil, false, errors.New("upload session operation namespace must match worker namespace")
	}

	request, plan, err := prepareRequest(operation, request)
	if err != nil {
		return nil, false, err
	}

	session, err := s.getSession(ctx, request.Request.Owner.UID)
	if err == nil {
		return session, false, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, false, err
	}

	secret, token, expiresAt, err := s.ensureSecret(ctx, operation, request.Request.Owner.UID)
	if err != nil {
		return nil, false, err
	}
	service, err := s.ensureService(ctx, operation, request.Request.Owner.UID)
	if err != nil {
		return nil, false, err
	}
	pod, err := s.ensurePod(ctx, operation, request, plan, secret.Name)
	if err != nil {
		return nil, false, err
	}

	return buildCreatedSession(pod, service, secret, token, expiresAt), true, nil
}

func (s *Service) deleteSession(ctx context.Context, session *Session) error {
	if session == nil {
		return nil
	}
	for _, object := range []client.Object{session.Pod, session.Service, session.Secret} {
		if object == nil {
			continue
		}
		if err := client.IgnoreNotFound(s.client.Delete(ctx, object)); err != nil {
			return err
		}
	}
	return nil
}

func prepareRequest(operation *corev1.ConfigMap, request publicationports.OperationContext) (publicationports.OperationContext, publicationapp.UploadSessionPlan, error) {
	request.OperationName = operation.Name
	request.OperationNamespace = operation.Namespace

	plan, err := requestPlan(request)
	if err != nil {
		return publicationports.OperationContext{}, publicationapp.UploadSessionPlan{}, err
	}
	return request, plan, nil
}

func uploadSessionHandleFromSession(service *Service, session *Session) *publicationports.UploadSessionHandle {
	if session == nil || session.Pod == nil {
		return nil
	}

	return publicationports.NewUploadSessionHandle(
		session.Pod.Name,
		session.Pod.Status.Phase,
		session.UploadStatus,
		func(ctx context.Context) error {
			return service.deleteSession(ctx, session)
		},
	)
}
