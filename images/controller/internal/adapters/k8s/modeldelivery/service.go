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

package modeldelivery

import (
	"context"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceOptions struct {
	Render       Options
	ManagedCache ManagedCacheOptions

	RegistrySourceNamespace      string
	RegistrySourceAuthSecretName string
	RegistrySourceCASecretName   string
	RuntimeImagePullSecretName   string
}

type ApplyRequest struct {
	Artifact        publication.PublishedArtifact
	ArtifactFamily  string
	TargetNamespace string
	Topology        TopologyHints
}

type ApplyResult struct {
	CacheMountPath    string
	ModelPath         string
	RegistryAccess    ociregistry.Projection
	ResolvedDigestKey string
	TopologyKind      CacheTopologyKind
	DeliveryMode      DeliveryMode
	DeliveryReason    DeliveryReason
}

type Service struct {
	client  client.Client
	scheme  *runtime.Scheme
	options ServiceOptions
}

func NewService(client client.Client, scheme *runtime.Scheme, options ServiceOptions) (*Service, error) {
	if client == nil {
		return nil, errors.New("runtime delivery service client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("runtime delivery service scheme must not be nil")
	}
	options = NormalizeServiceOptions(options)
	if err := validateServiceOptions(options); err != nil {
		return nil, err
	}

	return &Service{
		client:  client,
		scheme:  scheme,
		options: options,
	}, nil
}

func (s *Service) ApplyToPodTemplate(
	ctx context.Context,
	owner client.Object,
	request ApplyRequest,
	template *corev1.PodTemplateSpec,
) (ApplyResult, error) {
	if s == nil {
		return ApplyResult{}, errors.New("runtime delivery service must not be nil")
	}
	if owner == nil {
		return ApplyResult{}, errors.New("runtime delivery owner must not be nil")
	}
	if template == nil {
		return ApplyResult{}, errors.New("runtime delivery pod template must not be nil")
	}
	if err := ensureManagedCacheMount(template, s.options); err != nil {
		return ApplyResult{}, err
	}
	if err := validateTopologyHints(request.Topology); err != nil {
		return ApplyResult{}, err
	}

	topology, err := detectCacheTopology(
		template,
		request.Topology,
		s.options.Render.CacheMountPath,
		s.options.ManagedCache.VolumeName,
	)
	if err != nil {
		return ApplyResult{}, err
	}

	targetNamespace, err := resolveTargetNamespace(owner, request.TargetNamespace)
	if err != nil {
		return ApplyResult{}, err
	}
	coordination, err := s.resolveCoordination(ctx, targetNamespace, topology, request.Topology, request.Artifact.Digest)
	if err != nil {
		return ApplyResult{}, err
	}
	projection, err := ociregistry.EnsureProjectedAccessFromSourceNamespace(
		ctx,
		s.client,
		s.scheme,
		owner,
		targetNamespace,
		owner.GetUID(),
		s.options.RegistrySourceNamespace,
		s.options.RegistrySourceAuthSecretName,
		s.options.RegistrySourceCASecretName,
	)
	if err != nil {
		return ApplyResult{}, err
	}

	runtimeImagePullSecretName := ""
	if strings.TrimSpace(s.options.RuntimeImagePullSecretName) != "" {
		runtimeImagePullSecretName, err = ociregistry.EnsureProjectedImagePullSecretFromSourceNamespace(
			ctx,
			s.client,
			s.scheme,
			owner,
			targetNamespace,
			owner.GetUID(),
			s.options.RegistrySourceNamespace,
			s.options.RuntimeImagePullSecretName,
		)
		if err != nil {
			return ApplyResult{}, err
		}
	}

	rendered, err := Render(Input{
		Artifact:                   request.Artifact,
		ArtifactFamily:             request.ArtifactFamily,
		RegistryAccess:             projection,
		RuntimeImagePullSecretName: runtimeImagePullSecretName,
		CacheMount:                 topology.CacheMount,
		TopologyKind:               topology.Kind,
		Coordination:               coordination,
	}, s.options.Render)
	if err != nil {
		return ApplyResult{}, err
	}

	if err := applyRendered(template, rendered, request.Artifact.Digest, topology.DeliveryMode, topology.DeliveryReason); err != nil {
		return ApplyResult{}, err
	}

	return ApplyResult{
		CacheMountPath:    topology.CacheMount.MountPath,
		ModelPath:         rendered.ModelPath,
		RegistryAccess:    projection,
		ResolvedDigestKey: ResolvedDigestAnnotation,
		TopologyKind:      topology.Kind,
		DeliveryMode:      topology.DeliveryMode,
		DeliveryReason:    topology.DeliveryReason,
	}, nil
}

func NormalizeServiceOptions(options ServiceOptions) ServiceOptions {
	options.Render = NormalizeOptions(options.Render)
	options.ManagedCache = NormalizeManagedCacheOptions(options.ManagedCache)
	return options
}

func validateServiceOptions(options ServiceOptions) error {
	if err := ValidateOptions(options.Render); err != nil {
		return err
	}
	if err := ValidateManagedCacheOptions(options.ManagedCache); err != nil {
		return err
	}
	switch {
	case strings.TrimSpace(options.RegistrySourceNamespace) == "":
		return errors.New("runtime delivery registry source namespace must not be empty")
	case strings.TrimSpace(options.RegistrySourceAuthSecretName) == "":
		return errors.New("runtime delivery registry source auth secret name must not be empty")
	}
	return nil
}

func resolveTargetNamespace(owner client.Object, explicit string) (string, error) {
	if namespace := strings.TrimSpace(explicit); namespace != "" {
		return namespace, nil
	}
	if owner == nil {
		return "", errors.New("runtime delivery target namespace must not be empty")
	}
	if namespace := strings.TrimSpace(owner.GetNamespace()); namespace != "" {
		return namespace, nil
	}
	return "", errors.New("runtime delivery target namespace must not be empty")
}
