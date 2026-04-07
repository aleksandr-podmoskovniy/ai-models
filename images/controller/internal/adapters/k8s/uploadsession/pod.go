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
	"fmt"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	"github.com/deckhouse/ai-models/controller/internal/artifactbackend"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) ensurePod(
	ctx context.Context,
	owner client.Object,
	request publicationports.OperationContext,
	plan publicationapp.UploadSessionPlan,
	uploadTokenSecretName string,
) (*corev1.Pod, bool, error) {
	pod, err := s.buildPod(request, plan, uploadTokenSecretName)
	if err != nil {
		return nil, false, err
	}
	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, pod)
	if err != nil {
		return nil, false, err
	}
	return pod, created, nil
}

func (s *Service) buildPod(
	request publicationports.OperationContext,
	plan publicationapp.UploadSessionPlan,
	uploadTokenSecretName string,
) (*corev1.Pod, error) {
	name, err := resourcenames.UploadSessionPodName(request.Request.Owner.UID)
	if err != nil {
		return nil, err
	}
	serviceName, err := resourcenames.UploadSessionServiceName(request.Request.Owner.UID)
	if err != nil {
		return nil, err
	}
	artifactURI, err := artifactbackend.BuildOCIArtifactReference(s.options.OCIRepositoryPrefix, request.Request.Identity, request.Request.Owner.UID)
	if err != nil {
		return nil, err
	}

	env := append(
		ociregistry.Env(s.options.OCIInsecure, s.options.OCIRegistrySecretName, s.options.OCIRegistryCASecretName),
		corev1.EnvVar{
			Name: "AI_MODELS_UPLOAD_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: uploadTokenSecretName},
					Key:                  "token",
				},
			},
		},
	)

	volumeMounts := workloadpod.VolumeMounts(s.options.OCIRegistryCASecretName)
	volumes := workloadpod.Volumes(s.options.OCIRegistryCASecretName)

	args := []string{
		"upload-session",
		"--artifact-uri", artifactURI,
	}
	if plan.InputFormat != "" {
		args = append(args, "--input-format", string(plan.InputFormat))
	}
	if plan.ExpectedSizeBytes != nil && *plan.ExpectedSizeBytes > 0 {
		args = append(args, "--expected-size-bytes", fmt.Sprintf("%d", *plan.ExpectedSizeBytes))
	}
	if plan.Task != "" {
		args = append(args, "--task", plan.Task)
	}
	for _, engine := range plan.RuntimeEngines {
		args = append(args, "--runtime-engine", engine)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.options.Namespace,
			Labels: addServiceLabel(
				resourcenames.OwnerLabels("ai-models-upload", request.Request.Owner.Kind, request.Request.Owner.Name, request.Request.Owner.UID, request.Request.Owner.Namespace),
				serviceName,
			),
			Annotations: resourcenames.OwnerAnnotations(request.Request.Owner.Kind, request.Request.Owner.Name, request.Request.Owner.Namespace),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: s.options.ServiceAccountName,
			Volumes:            volumes,
			Containers: []corev1.Container{{
				Name:            "upload",
				Image:           s.options.Image,
				ImagePullPolicy: s.options.ImagePullPolicy,
				Args:            args,
				Env:             env,
				VolumeMounts:    volumeMounts,
				Ports: []corev1.ContainerPort{{
					Name:          "upload",
					ContainerPort: uploadPort,
					Protocol:      corev1.ProtocolTCP,
				}},
			}},
		},
	}, nil
}

func addServiceLabel(labels map[string]string, serviceName string) map[string]string {
	labels[serviceLabelKey] = serviceName
	return labels
}

func BuildPod(request publicationports.OperationContext, options Options, uploadTokenSecretName string) (*corev1.Pod, error) {
	options = normalizeOptions(options)
	if err := options.Validate(); err != nil {
		return nil, err
	}
	plan, err := requestPlan(request)
	if err != nil {
		return nil, err
	}
	service := &Service{options: options}
	return service.buildPod(request, plan, uploadTokenSecretName)
}

func requestPlan(request publicationports.OperationContext) (publicationapp.UploadSessionPlan, error) {
	task := ""
	var runtimeEngines []string
	if request.Request.Spec.RuntimeHints != nil {
		task = request.Request.Spec.RuntimeHints.Task
		for _, engine := range request.Request.Spec.RuntimeHints.Engines {
			runtimeEngines = append(runtimeEngines, string(engine))
		}
	}

	return publicationapp.IssueUploadSession(publicationapp.UploadSessionIssueRequest{
		OwnerUID:       string(request.Request.Owner.UID),
		OwnerKind:      request.Request.Owner.Kind,
		OwnerName:      request.Request.Owner.Name,
		Identity:       request.Request.Identity,
		Source:         request.Request.Spec.Source,
		InputFormat:    request.Request.Spec.InputFormat,
		Task:           task,
		RuntimeEngines: runtimeEngines,
	})
}
