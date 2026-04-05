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

package cleanupjob

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ociregistry"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Options struct {
	Namespace               string
	Image                   string
	ServiceAccountName      string
	OCIInsecure             bool
	OCIRegistrySecretName   string
	OCIRegistryCASecretName string
	Env                     []corev1.EnvVar
	ImagePullPolicy         corev1.PullPolicy
	TTLSecondsFinished      int32
}

type OwnerRef struct {
	UID       types.UID
	Kind      string
	Name      string
	Namespace string
}

func (o Options) Validate() error {
	if strings.TrimSpace(o.Namespace) == "" {
		return errors.New("cleanup job namespace must not be empty")
	}
	if strings.TrimSpace(o.Image) == "" {
		return errors.New("cleanup job image must not be empty")
	}
	if strings.TrimSpace(o.OCIRegistrySecretName) == "" {
		return errors.New("cleanup job OCI registry secret name must not be empty")
	}

	return nil
}

func BuildJob(owner OwnerRef, handle cleanuphandle.Handle, options Options) (*batchv1.Job, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if err := handle.Validate(); err != nil {
		return nil, err
	}

	name, err := resourcenames.CleanupJobName(owner.UID)
	if err != nil {
		return nil, err
	}

	backoffLimit := int32(0)
	ttlSeconds := options.TTLSecondsFinished
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	imagePullPolicy := options.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = corev1.PullIfNotPresent
	}
	handlePayload, err := json.Marshal(handle)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"app.kubernetes.io/name":            "ai-models-cleanup",
		"ai-models.deckhouse.io/owner-kind": resourcenames.TruncateLabelValue(owner.Kind),
		"ai-models.deckhouse.io/owner-name": resourcenames.TruncateLabelValue(owner.Name),
		"ai-models.deckhouse.io/owner-uid":  resourcenames.TruncateLabelValue(string(owner.UID)),
	}
	if owner.Namespace != "" {
		labels["ai-models.deckhouse.io/owner-namespace"] = resourcenames.TruncateLabelValue(owner.Namespace)
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: options.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: options.ServiceAccountName,
					Volumes:            volumesForOptions(options),
					Containers: []corev1.Container{
						{
							Name:            "cleanup",
							Image:           options.Image,
							ImagePullPolicy: imagePullPolicy,
							Command:         []string{"ai-models-backend-artifact-cleanup"},
							Args: []string{
								"--handle-json", string(handlePayload),
							},
							Env:          append(cleanupEnvForOptions(options), options.Env...),
							VolumeMounts: ociregistry.VolumeMounts(options.OCIRegistryCASecretName),
						},
					},
				},
			},
		},
	}, nil
}

func cleanupEnvForOptions(options Options) []corev1.EnvVar {
	return ociregistry.Env(options.OCIInsecure, options.OCIRegistrySecretName, options.OCIRegistryCASecretName)
}

func volumesForOptions(options Options) []corev1.Volume {
	return ociregistry.Volumes(options.OCIRegistryCASecretName)
}

func IsComplete(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

func IsFailed(job *batchv1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}
