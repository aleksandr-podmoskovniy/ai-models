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
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type deliverySignalState struct {
	Digest         string
	ArtifactURI    string
	ArtifactFamily string
	ModelPath      string
	DeliveryMode   string
	DeliveryReason string
}

func deliverySignalStateFromTemplate(template *corev1.PodTemplateSpec) deliverySignalState {
	if template == nil {
		return deliverySignalState{}
	}

	return deliverySignalState{
		Digest:         trimmedAnnotation(template.Annotations, modeldelivery.ResolvedDigestAnnotation),
		ArtifactURI:    trimmedAnnotation(template.Annotations, modeldelivery.ResolvedArtifactURIAnnotation),
		ArtifactFamily: trimmedAnnotation(template.Annotations, modeldelivery.ResolvedArtifactFamilyAnnotation),
		ModelPath:      managedRuntimeEnvValue(template.Spec.Containers, modeldelivery.ModelPathEnv),
		DeliveryMode:   trimmedAnnotation(template.Annotations, modeldelivery.ResolvedDeliveryModeAnnotation),
		DeliveryReason: trimmedAnnotation(template.Annotations, modeldelivery.ResolvedDeliveryReasonAnnotation),
	}
}

func (s deliverySignalState) empty() bool {
	return s == (deliverySignalState{})
}

func trimmedAnnotation(annotations map[string]string, key string) string {
	if len(annotations) == 0 {
		return ""
	}
	return strings.TrimSpace(annotations[key])
}

func managedRuntimeEnvValue(containers []corev1.Container, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for _, container := range containers {
		for _, env := range container.Env {
			if env.Name == name {
				return strings.TrimSpace(env.Value)
			}
		}
	}
	return ""
}

func newWorkloadObjectLike(object client.Object) (client.Object, error) {
	switch object.(type) {
	case *appsv1.Deployment:
		return &appsv1.Deployment{}, nil
	case *appsv1.StatefulSet:
		return &appsv1.StatefulSet{}, nil
	case *appsv1.DaemonSet:
		return &appsv1.DaemonSet{}, nil
	case *batchv1.CronJob:
		return &batchv1.CronJob{}, nil
	default:
		return nil, fmt.Errorf("unsupported workload delivery object type %T", object)
	}
}
