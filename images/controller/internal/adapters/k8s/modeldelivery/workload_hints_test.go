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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHintsForDeployment(t *testing.T) {
	t.Parallel()

	replicas := int32(3)
	hints := HintsForDeployment(&appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
	})

	if got, want := hints.ReplicaCount, int32(3); got != want {
		t.Fatalf("replica count = %d, want %d", got, want)
	}
	if len(hints.VolumeClaimTemplates) != 0 {
		t.Fatalf("expected no claim templates, got %#v", hints.VolumeClaimTemplates)
	}
}

func TestHintsForStatefulSet(t *testing.T) {
	t.Parallel()

	replicas := int32(4)
	hints := HintsForStatefulSet(&appsv1.StatefulSet{
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "model-cache"},
			}},
		},
	})

	if got, want := hints.ReplicaCount, int32(4); got != want {
		t.Fatalf("replica count = %d, want %d", got, want)
	}
	if got, want := len(hints.VolumeClaimTemplates), 1; got != want {
		t.Fatalf("claim template count = %d, want %d", got, want)
	}
	if got, want := hints.VolumeClaimTemplates[0].Name, "model-cache"; got != want {
		t.Fatalf("claim template name = %q, want %q", got, want)
	}
}

func TestHintsForDaemonSet(t *testing.T) {
	t.Parallel()

	hints := HintsForDaemonSet(&appsv1.DaemonSet{})
	if got, want := hints.ReplicaCount, multiplePodsHint; got != want {
		t.Fatalf("replica count = %d, want %d", got, want)
	}
}

func TestHintsForJobAndCronJob(t *testing.T) {
	t.Parallel()

	parallelism := int32(5)
	jobHints := HintsForJob(&batchv1.Job{
		Spec: batchv1.JobSpec{
			Parallelism: &parallelism,
		},
	})
	if got, want := jobHints.ReplicaCount, int32(5); got != want {
		t.Fatalf("job replica count = %d, want %d", got, want)
	}

	cronHints := HintsForCronJob(&batchv1.CronJob{
		Spec: batchv1.CronJobSpec{
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Parallelism: &parallelism,
				},
			},
		},
	})
	if got, want := cronHints.ReplicaCount, int32(5); got != want {
		t.Fatalf("cronjob replica count = %d, want %d", got, want)
	}
}

func TestHintsFallbackToSingleReplica(t *testing.T) {
	t.Parallel()

	if got, want := HintsForDeployment(nil).ReplicaCount, int32(1); got != want {
		t.Fatalf("nil deployment replica count = %d, want %d", got, want)
	}
	if got, want := HintsForJob(&batchv1.Job{}).ReplicaCount, int32(1); got != want {
		t.Fatalf("default job replica count = %d, want %d", got, want)
	}
}
