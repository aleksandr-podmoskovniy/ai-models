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

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceOptions struct {
	Render          Options
	ManagedCache    ManagedCacheOptions
	SharedPVC       SharedPVCOptions
	DeliveryAuthKey string

	RegistrySourceNamespace string
}

type ApplyRequest struct {
	Artifact       publication.PublishedArtifact
	ArtifactFamily string
	Bindings       []ModelBinding
	Topology       TopologyHints
}

type ModelBinding struct {
	Name           string
	Artifact       publication.PublishedArtifact
	ArtifactFamily string
}

type ApplyResult struct {
	CacheMountPath    string
	ModelPath         string
	ResolvedDigestKey string
	TopologyKind      CacheTopologyKind
	DeliveryMode      DeliveryMode
	DeliveryReason    DeliveryReason
	GateReason        DeliveryGateReason
	GateMessage       string
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
	input, topology, pvcState, err := s.renderInput(ctx, owner, request, template)
	if err != nil {
		return ApplyResult{}, err
	}
	gate, err := s.deliveryGateForTemplate(topology, input, pvcState)
	if err != nil {
		return ApplyResult{}, err
	}

	rendered, err := Render(input, s.options.Render)
	if err != nil {
		return ApplyResult{}, err
	}

	if err := applyRendered(template, owner.GetNamespace(), rendered, input.Artifact.Digest, topology.DeliveryMode, topology.DeliveryReason, s.options.DeliveryAuthKey); err != nil {
		return ApplyResult{}, err
	}
	s.pruneManagedCacheTemplateState(template, topology, rendered)
	applyDeliverySchedulingGate(template, topology.Kind, gate.Ready)

	return ApplyResult{
		CacheMountPath:    topology.CacheMount.MountPath,
		ModelPath:         rendered.ModelPath,
		ResolvedDigestKey: ResolvedDigestAnnotation,
		TopologyKind:      topology.Kind,
		DeliveryMode:      topology.DeliveryMode,
		DeliveryReason:    topology.DeliveryReason,
		GateReason:        gate.Reason,
		GateMessage:       gate.Message,
	}, nil
}

func (s *Service) renderInput(
	ctx context.Context,
	owner client.Object,
	request ApplyRequest,
	template *corev1.PodTemplateSpec,
) (Input, CacheTopology, sharedPVCState, error) {
	bindings, _, err := normalizeApplyBindings(request)
	if err != nil {
		return Input{}, CacheTopology{}, sharedPVCState{}, err
	}
	if err := ensureManagedCacheTemplate(template, s.options, bindings); err != nil {
		return Input{}, CacheTopology{}, sharedPVCState{}, err
	}

	shared := NormalizeSharedPVCOptions(s.options.SharedPVC)
	sharedClaimName := ""
	if !s.options.ManagedCache.Enabled && shared.StorageClassName != "" {
		sharedClaimName = sharedPVCClaimName(owner, bindings)
	}
	topology, err := detectApplyTopology(
		template,
		request.Topology,
		s.options.Render.CacheMountPath,
		s.options.ManagedCache.VolumeName,
		s.options.ManagedCache.Enabled,
		sharedClaimName,
		shared.VolumeName,
	)
	if err != nil {
		return Input{}, CacheTopology{}, sharedPVCState{}, err
	}
	pvcState, err := s.ensureSharedPVC(ctx, owner, bindings, topology.ClaimName)
	if err != nil {
		return Input{}, CacheTopology{}, sharedPVCState{}, err
	}
	input, err := s.resolveRenderInput(owner, bindings, topology)
	if err != nil {
		return Input{}, CacheTopology{}, sharedPVCState{}, err
	}
	return input, topology, pvcState, nil
}

func (s *Service) resolveRenderInput(
	owner client.Object,
	bindings []ModelBinding,
	topology CacheTopology,
) (Input, error) {
	legacyImagePullSecretName, err := legacyRuntimeImagePullSecretName(owner, topology.Kind)
	if err != nil {
		return Input{}, err
	}
	return Input{
		Artifact:                  bindings[0].Artifact,
		ArtifactFamily:            bindings[0].ArtifactFamily,
		Bindings:                  inputBindings(bindings),
		LegacyImagePullSecretName: legacyImagePullSecretName,
		CacheMount:                topology.CacheMount,
		SharedPVCClaimName:        topology.ClaimName,
		TopologyKind:              topology.Kind,
	}, nil
}

func applyDeliverySchedulingGate(template *corev1.PodTemplateSpec, topologyKind CacheTopologyKind, ready bool) {
	if !ready {
		EnsureSchedulingGate(template)
	}
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

func legacyRuntimeImagePullSecretName(owner client.Object, topologyKind CacheTopologyKind) (string, error) {
	if topologyKind != CacheTopologyDirect {
		return "", nil
	}
	return resourcenames.RuntimeImagePullSecretName(owner.GetUID())
}

func NormalizeServiceOptions(options ServiceOptions) ServiceOptions {
	options.Render = NormalizeOptions(options.Render)
	options.ManagedCache = NormalizeManagedCacheOptions(options.ManagedCache)
	options.SharedPVC = NormalizeSharedPVCOptions(options.SharedPVC)
	options.DeliveryAuthKey = strings.TrimSpace(options.DeliveryAuthKey)
	return options
}

func validateServiceOptions(options ServiceOptions) error {
	if err := ValidateOptions(options.Render); err != nil {
		return err
	}
	if err := ValidateManagedCacheOptions(options.ManagedCache); err != nil {
		return err
	}
	if err := ValidateSharedPVCOptions(options.SharedPVC); err != nil {
		return err
	}
	switch {
	case options.ManagedCache.Enabled && strings.TrimSpace(options.DeliveryAuthKey) == "":
		return errors.New("runtime delivery auth key must not be empty when managed node-cache delivery is enabled")
	case strings.TrimSpace(options.RegistrySourceNamespace) == "":
		return errors.New("runtime delivery registry source namespace must not be empty")
	}
	return nil
}
