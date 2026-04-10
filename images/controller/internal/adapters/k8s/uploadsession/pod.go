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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/objectstorage"
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
	options Options,
) (*corev1.Pod, string, bool, error) {
	pod, artifactURI, err := s.buildPod(request, plan, uploadTokenSecretName, options)
	if err != nil {
		return nil, "", false, err
	}
	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, pod)
	if err != nil {
		return nil, "", false, err
	}
	return pod, artifactURI, created, nil
}

func (s *Service) buildPod(
	request publicationports.OperationContext,
	plan publicationapp.UploadSessionPlan,
	uploadTokenSecretName string,
	options Options,
) (*corev1.Pod, string, error) {
	name, err := resourcenames.UploadSessionPodName(request.Request.Owner.UID)
	if err != nil {
		return nil, "", err
	}
	serviceName, err := resourcenames.UploadSessionServiceName(request.Request.Owner.UID)
	if err != nil {
		return nil, "", err
	}
	artifactURI, err := artifactbackend.BuildOCIArtifactReference(options.Runtime.OCIRepositoryPrefix, request.Request.Identity, request.Request.Owner.UID)
	if err != nil {
		return nil, "", err
	}
	stagingPrefix, err := resourcenames.UploadStagingObjectPrefix(request.Request.Owner.UID)
	if err != nil {
		return nil, "", err
	}

	env := append(
		objectstorage.Env(options.Runtime.ObjectStorage),
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

	volumeMounts := workloadpod.VolumeMounts("", objectstorage.VolumeMounts(options.Runtime.ObjectStorage.CASecretName)...)
	volumes := workloadpod.Volumes("", objectstorage.Volumes(options.Runtime.ObjectStorage.CASecretName)...)

	args := []string{
		"upload-session",
		"--staging-bucket", options.Runtime.ObjectStorage.Bucket,
		"--staging-key-prefix", stagingPrefix,
	}
	if plan.ExpectedSizeBytes != nil && *plan.ExpectedSizeBytes > 0 {
		args = append(args, "--expected-size-bytes", fmt.Sprintf("%d", *plan.ExpectedSizeBytes))
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: s.options.Runtime.Namespace,
			Labels: addServiceLabel(
				resourcenames.OwnerLabels("ai-models-upload", request.Request.Owner.Kind, request.Request.Owner.Name, request.Request.Owner.UID, request.Request.Owner.Namespace),
				serviceName,
			),
			Annotations: resourcenames.OwnerAnnotations(request.Request.Owner.Kind, request.Request.Owner.Name, request.Request.Owner.Namespace),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: options.Runtime.ServiceAccountName,
			Volumes:            volumes,
			Containers: []corev1.Container{{
				Name:            "upload",
				Image:           options.Runtime.Image,
				ImagePullPolicy: options.Runtime.ImagePullPolicy,
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
	}, artifactURI, nil
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
	pod, _, err := service.buildPod(request, plan, uploadTokenSecretName, options)
	return pod, err
}

func requestPlan(request publicationports.OperationContext) (publicationapp.UploadSessionPlan, error) {
	return publicationapp.IssueUploadSession(publicationapp.UploadSessionIssueRequest{
		OwnerUID:  string(request.Request.Owner.UID),
		OwnerKind: request.Request.Owner.Kind,
		OwnerName: request.Request.Owner.Name,
		Identity:  request.Request.Identity,
		Source:    request.Request.Spec.Source,
	})
}
