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
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/directuploadstate"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
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
	options = normalizeOptions(options)
	if err := validateServiceOptions(options); err != nil {
		return nil, err
	}

	return &Service{
		client:  client,
		scheme:  scheme,
		options: options,
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
	directUploadStateSecret, directUploadState, err := s.prepareRequestState(ctx, owner, request, plan)
	if err != nil {
		return nil, false, err
	}
	if handle, done, err := s.existingOrQueuedHandle(ctx, request.Owner.UID, directUploadState); err != nil {
		return nil, false, err
	} else if done {
		return handle, false, nil
	}

	options, projectedAuthSecretName, err := s.prepareProjectedDependencies(ctx, owner, request.Owner, plan)
	if err != nil {
		return nil, false, err
	}

	pod, err := buildWithPlan(request, plan, options, projectedAuthSecretName, directUploadStateSecret.Name)
	if err != nil {
		return nil, false, err
	}

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, pod)
	if err != nil {
		return nil, false, err
	}

	return s.handleFromPod(pod, directUploadState), created, nil
}

func (s *Service) handleFromPod(
	pod *corev1.Pod,
	directUploadState modelpackports.DirectUploadState,
) *publicationports.SourceWorkerHandle {
	message := terminationMessage(pod, "publish")
	if message == "" && directUploadState.Phase == modelpackports.DirectUploadStatePhaseFailed {
		message = strings.TrimSpace(directUploadState.FailureMessage)
	}
	progress := directUploadProgress(directUploadState)
	return publicationports.NewSourceWorkerHandle(
		pod.Name,
		pod.Status.Phase,
		message,
		progress.Reason,
		progress.Progress,
		progress.Message,
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
	return publicationports.NewSourceWorkerHandle(name, corev1.PodPending, "", modelsv1alpha1.ModelConditionReasonPending, "", "", nil), false, nil
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

func (s *Service) ensureDirectUploadStateSecret(
	ctx context.Context,
	owner client.Object,
	requestOwner publicationports.Owner,
) (*corev1.Secret, error) {
	ownerGeneration := normalizedOwnerGeneration(owner.GetGeneration())
	name, err := resourcenames.SourceWorkerStateSecretName(requestOwner.UID)
	if err != nil {
		return nil, err
	}

	secret, err := directuploadstate.NewSecret(directuploadstate.SecretSpec{
		Name:            name,
		Namespace:       s.options.Namespace,
		OwnerGeneration: ownerGeneration,
	})
	if err != nil {
		return nil, err
	}
	secret.Labels = mergeStringMaps(
		secret.Labels,
		resourcenames.OwnerLabels("ai-models-publication-state", requestOwner.Kind, requestOwner.Name, requestOwner.UID, requestOwner.Namespace),
	)
	secret.Annotations = mergeStringMaps(
		secret.Annotations,
		resourcenames.OwnerAnnotations(requestOwner.Kind, requestOwner.Name, requestOwner.Namespace),
	)

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, secret)
	if err != nil {
		return nil, err
	}
	if created {
		return secret, nil
	}

	recordedGeneration, err := directuploadstate.OwnerGenerationFromSecret(secret)
	if err != nil {
		return nil, err
	}
	if recordedGeneration == normalizedOwnerGeneration(owner.GetGeneration()) {
		return secret, nil
	}

	if err := ownedresource.DeleteAll(ctx, s.client, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace},
	}); err != nil {
		return nil, err
	}

	recreated, err := directuploadstate.NewSecret(directuploadstate.SecretSpec{
		Name:            name,
		Namespace:       s.options.Namespace,
		OwnerGeneration: normalizedOwnerGeneration(owner.GetGeneration()),
	})
	if err != nil {
		return nil, err
	}
	recreated.Labels = mergeStringMaps(
		recreated.Labels,
		resourcenames.OwnerLabels("ai-models-publication-state", requestOwner.Kind, requestOwner.Name, requestOwner.UID, requestOwner.Namespace),
	)
	recreated.Annotations = mergeStringMaps(
		recreated.Annotations,
		resourcenames.OwnerAnnotations(requestOwner.Kind, requestOwner.Name, requestOwner.Namespace),
	)
	if _, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, recreated); err != nil {
		return nil, err
	}
	return recreated, nil
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]string, len(extra))
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}

func normalizedOwnerGeneration(generation int64) int64 {
	if generation > 0 {
		return generation
	}
	return 1
}
