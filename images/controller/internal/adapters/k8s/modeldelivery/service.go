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
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
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
	Bindings        []ModelBinding
	TargetNamespace string
	Topology        TopologyHints
}

type ModelBinding struct {
	Alias          string
	Artifact       publication.PublishedArtifact
	ArtifactFamily string
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
	if err := validateApplyInputs(s, owner, template, request.Topology); err != nil {
		return ApplyResult{}, err
	}
	input, topology, aliasContract, err := s.renderInput(ctx, owner, request, template)
	if err != nil {
		return ApplyResult{}, err
	}
	readyNodes, err := s.readyNodesForTopology(ctx, topology.Kind)
	if err != nil {
		return ApplyResult{}, err
	}

	rendered, err := Render(input, s.options.Render)
	if err != nil {
		return ApplyResult{}, err
	}

	if err := applyRendered(template, rendered, input.Artifact.Digest, topology.DeliveryMode, topology.DeliveryReason); err != nil {
		return ApplyResult{}, err
	}
	s.pruneManagedCacheTemplateState(template, topology, rendered, aliasContract)
	applyReadyNodeSchedulingGate(template, topology.Kind, readyNodes)

	return ApplyResult{
		CacheMountPath:    topology.CacheMount.MountPath,
		ModelPath:         rendered.ModelPath,
		RegistryAccess:    input.RegistryAccess,
		ResolvedDigestKey: ResolvedDigestAnnotation,
		TopologyKind:      topology.Kind,
		DeliveryMode:      topology.DeliveryMode,
		DeliveryReason:    topology.DeliveryReason,
	}, nil
}

func (s *Service) renderInput(
	ctx context.Context,
	owner client.Object,
	request ApplyRequest,
	template *corev1.PodTemplateSpec,
) (Input, CacheTopology, bool, error) {
	bindings, aliasContract, err := normalizeApplyBindings(request)
	if err != nil {
		return Input{}, CacheTopology{}, false, err
	}
	if err := ensureManagedCacheTemplate(template, s.options, bindings, aliasContract); err != nil {
		return Input{}, CacheTopology{}, false, err
	}

	topology, err := detectApplyTopology(
		template,
		request.Topology,
		s.options.Render.CacheMountPath,
		s.options.ManagedCache.VolumeName,
		aliasContract && s.options.ManagedCache.Enabled,
	)
	if err != nil {
		return Input{}, CacheTopology{}, false, err
	}
	input, err := s.resolveRenderInput(ctx, owner, request, bindings, aliasContract, topology)
	if err != nil {
		return Input{}, CacheTopology{}, false, err
	}
	return input, topology, aliasContract, nil
}

func (s *Service) resolveRenderInput(
	ctx context.Context,
	owner client.Object,
	request ApplyRequest,
	bindings []ModelBinding,
	aliasContract bool,
	topology CacheTopology,
) (Input, error) {
	targetNamespace, err := resolveTargetNamespace(owner, request.TargetNamespace)
	if err != nil {
		return Input{}, err
	}
	if err := s.prepareTopologyAccess(ctx, owner, targetNamespace, topology.Kind); err != nil {
		return Input{}, err
	}
	coordination, err := s.resolveCoordination(ctx, targetNamespace, topology, request.Topology, bindings[0].Artifact.Digest)
	if err != nil {
		return Input{}, err
	}
	projection, err := s.ensureRegistryProjection(ctx, owner, targetNamespace, topology.Kind)
	if err != nil {
		return Input{}, err
	}
	runtimeImagePullSecretName, err := s.runtimeImagePullSecretName(ctx, owner, targetNamespace, topology.Kind)
	if err != nil {
		return Input{}, err
	}
	return Input{
		Artifact:                   bindings[0].Artifact,
		ArtifactFamily:             bindings[0].ArtifactFamily,
		Bindings:                   inputBindings(bindings, aliasContract),
		RegistryAccess:             projection,
		RuntimeImagePullSecretName: runtimeImagePullSecretName,
		CacheMount:                 topology.CacheMount,
		TopologyKind:               topology.Kind,
		Coordination:               coordination,
	}, nil
}

func (s *Service) prepareTopologyAccess(ctx context.Context, owner client.Object, targetNamespace string, topologyKind CacheTopologyKind) error {
	if topologyKind != CacheTopologyDirect {
		return nil
	}
	return s.cleanupProjectedRuntimeAccess(ctx, owner, targetNamespace)
}

func (s *Service) readyNodesForTopology(ctx context.Context, topologyKind CacheTopologyKind) (bool, error) {
	if topologyKind != CacheTopologyDirect {
		return true, nil
	}
	return s.hasManagedCacheReadyNode(ctx)
}

func applyReadyNodeSchedulingGate(template *corev1.PodTemplateSpec, topologyKind CacheTopologyKind, readyNodes bool) {
	if topologyKind == CacheTopologyDirect && !readyNodes {
		EnsureSchedulingGate(template)
	}
}

func (s *Service) hasManagedCacheReadyNode(ctx context.Context) (bool, error) {
	managed := NormalizeManagedCacheOptions(s.options.ManagedCache)
	if !managed.Enabled || len(managed.NodeSelector) == 0 {
		return true, nil
	}
	nodes := &corev1.NodeList{}
	if err := s.client.List(ctx, nodes, client.MatchingLabels(managed.NodeSelector)); err != nil {
		return false, err
	}
	return len(nodes.Items) > 0, nil
}

func validateApplyInputs(s *Service, owner client.Object, template *corev1.PodTemplateSpec, topology TopologyHints) error {
	switch {
	case s == nil:
		return errors.New("runtime delivery service must not be nil")
	case owner == nil:
		return errors.New("runtime delivery owner must not be nil")
	case template == nil:
		return errors.New("runtime delivery pod template must not be nil")
	default:
		return validateTopologyHints(topology)
	}
}

func (s *Service) ensureRegistryProjection(ctx context.Context, owner client.Object, targetNamespace string, topologyKind CacheTopologyKind) (ociregistry.Projection, error) {
	if topologyKind == CacheTopologyDirect {
		return ociregistry.Projection{}, nil
	}
	return ociregistry.EnsureProjectedAccessFromSourceNamespace(
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
}

func (s *Service) runtimeImagePullSecretName(ctx context.Context, owner client.Object, targetNamespace string, topologyKind CacheTopologyKind) (string, error) {
	if topologyKind == CacheTopologyDirect {
		return resourcenames.RuntimeImagePullSecretName(owner.GetUID())
	}
	if strings.TrimSpace(s.options.RuntimeImagePullSecretName) == "" {
		return "", nil
	}
	return ociregistry.EnsureProjectedImagePullSecretFromSourceNamespace(
		ctx,
		s.client,
		s.scheme,
		owner,
		targetNamespace,
		owner.GetUID(),
		s.options.RegistrySourceNamespace,
		s.options.RuntimeImagePullSecretName,
	)
}

func (s *Service) cleanupProjectedRuntimeAccess(ctx context.Context, owner client.Object, targetNamespace string) error {
	if err := ociregistry.DeleteProjectedAccess(ctx, s.client, targetNamespace, owner.GetUID()); err != nil {
		return err
	}
	return ociregistry.DeleteProjectedImagePullSecret(ctx, s.client, targetNamespace, owner.GetUID())
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
