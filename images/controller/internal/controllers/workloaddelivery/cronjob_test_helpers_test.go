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
	"log/slog"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newCronJobReconciler(t *testing.T, objects ...client.Object) (*baseReconciler, client.Client) {
	t.Helper()

	serviceOptions := modeldelivery.ServiceOptions{
		Render: modeldelivery.Options{
			RuntimeImage: "example.com/ai-models/controller-runtime:dev",
		},
		RegistrySourceNamespace:      testRegistryNamespace,
		RegistrySourceAuthSecretName: testRegistryAuthName,
		RuntimeImagePullSecretName:   testRuntimePullSecret,
	}
	scheme := testkit.NewScheme(t, batchv1.AddToScheme)
	objects = append(objects, testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, serviceOptions.RuntimeImagePullSecretName))
	kubeClient := testkit.NewFakeClient(t, scheme, nil, objects...)
	service, err := modeldelivery.NewService(kubeClient, scheme, serviceOptions)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	return &baseReconciler{
		client:   kubeClient,
		reader:   kubeClient,
		delivery: service,
		options:  Options{Service: serviceOptions},
		logger:   slog.Default(),
		recorder: record.NewFakeRecorder(16),
	}, kubeClient
}

func annotatedCronJob(annotations map[string]string, volumeSource corev1.VolumeSource) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "runtime",
			Namespace:   "team-a",
			UID:         types.UID("660e8400-e29b-41d4-a716-446655440999"),
			Annotations: annotations,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "* * * * *",
			JobTemplate: batchv1.JobTemplateSpec{Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "runtime"}},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "runtime",
							Image: "example.com/runtime:dev",
							VolumeMounts: []corev1.VolumeMount{{
								Name:      "model-cache",
								MountPath: modeldelivery.DefaultCacheMountPath,
							}},
						}},
						Volumes: []corev1.Volume{{
							Name:         "model-cache",
							VolumeSource: volumeSource,
						}},
					},
				},
			}},
		},
	}
}

func reconcileCronJob(t *testing.T, reconciler *baseReconciler, workload *batchv1.CronJob) ctrl.Result {
	t.Helper()

	result, err := reconciler.reconcileWorkload(context.Background(), workload)
	if err != nil {
		t.Fatalf("reconcileWorkload() error = %v", err)
	}
	return result
}
