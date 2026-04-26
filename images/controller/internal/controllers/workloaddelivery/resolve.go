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

package workloaddelivery

import (
	"context"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ResolvedBinding struct {
	Alias     string
	Reference Reference
	Artifact  publication.PublishedArtifact
	Family    string
}

type Resolution struct {
	Artifact publication.PublishedArtifact
	Family   string
	Bindings []ResolvedBinding
	Ready    bool
	Message  string
}

func (r *baseReconciler) resolveReferences(ctx context.Context, namespace string, refs []Reference) (Resolution, error) {
	if len(refs) == 0 {
		return Resolution{}, fmt.Errorf("workload delivery requires at least one model reference")
	}
	bindings := make([]ResolvedBinding, 0, len(refs))
	for _, ref := range refs {
		resolution, err := r.resolveSingleReference(ctx, namespace, ref)
		if err != nil || !resolution.Ready {
			return resolution, err
		}
		bindings = append(bindings, ResolvedBinding{
			Alias:     ref.Alias,
			Reference: ref,
			Artifact:  resolution.Artifact,
			Family:    resolution.Family,
		})
	}
	return Resolution{
		Artifact: bindings[0].Artifact,
		Family:   bindings[0].Family,
		Bindings: bindings,
		Ready:    true,
	}, nil
}

func (r *baseReconciler) resolveSingleReference(ctx context.Context, namespace string, ref Reference) (Resolution, error) {
	switch ref.Scope {
	case ReferenceScopeModel:
		object := &modelsv1alpha1.Model{}
		key := client.ObjectKey{Namespace: namespace, Name: ref.Name}
		if err := r.client.Get(ctx, key, object); err != nil {
			return Resolution{Message: fmt.Sprintf("referenced Model %s/%s is unavailable", namespace, ref.Name)}, client.IgnoreNotFound(err)
		}
		return resolutionFromStatus(object.Status, "Model", key.String())
	case ReferenceScopeClusterModel:
		object := &modelsv1alpha1.ClusterModel{}
		key := client.ObjectKey{Name: ref.Name}
		if err := r.client.Get(ctx, key, object); err != nil {
			return Resolution{Message: fmt.Sprintf("referenced ClusterModel %s is unavailable", ref.Name)}, client.IgnoreNotFound(err)
		}
		return resolutionFromStatus(object.Status, "ClusterModel", ref.Name)
	default:
		return Resolution{}, fmt.Errorf("unsupported workload delivery reference scope %q", ref.Scope)
	}
}

func (r Resolution) modelDeliveryBindings(aliasContract bool) []modeldelivery.ModelBinding {
	if !aliasContract {
		return nil
	}
	bindings := make([]modeldelivery.ModelBinding, 0, len(r.Bindings))
	for _, binding := range r.Bindings {
		bindings = append(bindings, modeldelivery.ModelBinding{
			Alias:          binding.Alias,
			Artifact:       binding.Artifact,
			ArtifactFamily: binding.Family,
		})
	}
	return bindings
}

func (r Resolution) modelCount() int {
	if len(r.Bindings) > 0 {
		return len(r.Bindings)
	}
	if strings.TrimSpace(r.Artifact.Digest) != "" {
		return 1
	}
	return 0
}

func resolutionFromStatus(status modelsv1alpha1.ModelStatus, kind, name string) (Resolution, error) {
	if status.Phase != modelsv1alpha1.ModelPhaseReady {
		return Resolution{
			Ready:   false,
			Message: fmt.Sprintf("referenced %s %s is not Ready", kind, name),
		}, nil
	}
	if status.Artifact == nil {
		return Resolution{
			Ready:   false,
			Message: fmt.Sprintf("referenced %s %s has no published artifact", kind, name),
		}, nil
	}

	artifact := publication.PublishedArtifact{
		Kind:      status.Artifact.Kind,
		URI:       strings.TrimSpace(status.Artifact.URI),
		Digest:    strings.TrimSpace(status.Artifact.Digest),
		MediaType: strings.TrimSpace(status.Artifact.MediaType),
	}
	if status.Artifact.SizeBytes != nil {
		artifact.SizeBytes = *status.Artifact.SizeBytes
	}
	if err := artifact.Validate(); err != nil {
		return Resolution{
			Ready:   false,
			Message: fmt.Sprintf("referenced %s %s has invalid published artifact: %v", kind, name, err),
		}, nil
	}
	if strings.TrimSpace(artifact.Digest) == "" {
		return Resolution{
			Ready:   false,
			Message: fmt.Sprintf("referenced %s %s has empty artifact digest", kind, name),
		}, nil
	}

	resolution := Resolution{
		Artifact: artifact,
		Ready:    true,
	}
	if status.Resolved != nil {
		resolution.Family = strings.TrimSpace(status.Resolved.Family)
	}
	return resolution, nil
}
