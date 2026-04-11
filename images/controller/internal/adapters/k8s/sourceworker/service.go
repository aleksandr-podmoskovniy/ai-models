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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	"github.com/deckhouse/ai-models/controller/internal/application/sourceadmission"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client          client.Client
	scheme          *runtime.Scheme
	options         Options
	httpSourceProbe sourceadmission.HTTPSourceProber
}

func NewService(client client.Client, scheme *runtime.Scheme, options Options) (*Service, error) {
	if client == nil {
		return nil, errors.New("source worker service client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("source worker service scheme must not be nil")
	}
	options = normalizeOptions(options)
	if err := validateServiceOptions(options); err != nil {
		return nil, err
	}

	return &Service{
		client:          client,
		scheme:          scheme,
		options:         options,
		httpSourceProbe: httpSourcePreflightProber{},
	}, nil
}

func (s *Service) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.SourceWorkerHandle, bool, error) {
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
	if err := s.preflight(ctx, request, plan); err != nil {
		return nil, false, err
	}
	if existingPod, found, err := s.lookupPod(ctx, request.Owner.UID); err != nil {
		return nil, false, err
	} else if found {
		return s.handleFromPod(existingPod), false, nil
	}
	if blocked, err := s.publishConcurrencyBlocked(ctx); err != nil {
		return nil, false, err
	} else if blocked {
		return queuedHandle(request.Owner.UID)
	}

	projectedAuthSecretName, err := s.ensureProjectedAuthSecret(ctx, owner, request.Owner, plan)
	if err != nil {
		return nil, false, err
	}
	projection, err := ociregistry.EnsureProjectedAccess(
		ctx,
		s.client,
		s.scheme,
		owner,
		s.options.Namespace,
		request.Owner.UID,
		s.options.OCIRegistrySecretName,
		s.options.OCIRegistryCASecretName,
	)
	if err != nil {
		return nil, false, err
	}
	options := s.options
	options.OCIRegistrySecretName = projection.AuthSecretName
	options.OCIRegistryCASecretName = projection.CASecretName

	pod, err := buildWithPlan(request, plan, options, projectedAuthSecretName)
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
	ownerUID, ok := resourcenames.OwnerUIDFromLabels(pod.Labels)
	if ok {
		if err := ociregistry.DeleteProjectedAccess(ctx, s.client, s.options.Namespace, ownerUID); err != nil {
			return err
		}
	}
	secret, err := s.projectedAuthSecretForPod(pod)
	if err != nil {
		return err
	}
	return ownedresource.DeleteAll(ctx, s.client, secret, pod)
}

func (s *Service) lookupPod(ctx context.Context, ownerUID types.UID) (*corev1.Pod, bool, error) {
	name, err := resourcenames.SourceWorkerPodName(ownerUID)
	if err != nil {
		return nil, false, err
	}

	var pod corev1.Pod
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: s.options.Namespace}, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &pod, true, nil
}

func (s *Service) publishConcurrencyBlocked(ctx context.Context) (bool, error) {
	var pods corev1.PodList
	if err := s.client.List(ctx, &pods,
		client.InNamespace(s.options.Namespace),
		client.MatchingLabels{resourcenames.AppNameLabelKey: "ai-models-publication"},
	); err != nil {
		return false, err
	}

	active := 0
	for i := range pods.Items {
		if isActiveWorkerPhase(pods.Items[i].Status.Phase) {
			active++
		}
	}

	return active >= s.options.MaxConcurrentWorkers, nil
}

func queuedHandle(ownerUID types.UID) (*publicationports.SourceWorkerHandle, bool, error) {
	name, err := resourcenames.SourceWorkerPodName(ownerUID)
	if err != nil {
		return nil, false, err
	}
	return publicationports.NewSourceWorkerHandle(name, corev1.PodPending, "", nil), false, nil
}

func isActiveWorkerPhase(phase corev1.PodPhase) bool {
	switch phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return false
	default:
		return true
	}
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
