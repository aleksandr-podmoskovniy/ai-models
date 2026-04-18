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

package catalogcleanup

import (
	"context"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func cleanupJobName(t *testing.T, object client.Object) string {
	t.Helper()

	name, err := resourcenames.CleanupJobName(object.GetUID())
	if err != nil {
		t.Fatalf("CleanupJobName() error = %v", err)
	}

	return name
}

func runningJob(namespace, name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func completedJob(namespace, name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			},
		},
	}
}

func requestedGCSecret(namespace string, ownerUID types.UID) *corev1.Secret {
	secret := buildDMCRGCRequestSecret(namespace, cleanupJobOwner{
		UID:  ownerUID,
		Kind: modelsv1alpha1.ModelKind,
		Name: "deepseek-r1",
	})
	return secret
}

func activeGCSecret(namespace string, ownerUID types.UID) *corev1.Secret {
	secret := requestedGCSecret(namespace, ownerUID)
	delete(secret.Annotations, dmcrGCRequestedAnnotationKey)
	secret.Annotations[dmcrGCSwitchAnnotationKey] = time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
	return secret
}

func completedGCSecret(namespace string, ownerUID types.UID) *corev1.Secret {
	secret := activeGCSecret(namespace, ownerUID)
	secret.Annotations[dmcrGCDoneAnnotationKey] = time.Now().UTC().Format(dmcrGCRequestTimestampRFC)
	return secret
}

func failedJob(namespace, name string) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
			},
		},
	}
}

func assertCleanupJobExists(t *testing.T, kubeClient client.Client, name string) {
	t.Helper()

	var job batchv1.Job
	if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: "d8-ai-models", Name: name}, &job); err != nil {
		t.Fatalf("expected cleanup job %q, got err=%v", name, err)
	}
}

func assertCleanupCondition(
	t *testing.T,
	kubeClient client.Client,
	object client.Object,
	expectedPhase modelsv1alpha1.ModelPhase,
	expectedReason modelsv1alpha1.ModelConditionReason,
) {
	t.Helper()

	updated := &modelsv1alpha1.Model{}
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(object), updated); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.Status.Phase != expectedPhase {
		t.Fatalf("phase = %q, want %q", updated.Status.Phase, expectedPhase)
	}

	var readyCondition *metav1.Condition
	for i := range updated.Status.Conditions {
		if updated.Status.Conditions[i].Type == string(modelsv1alpha1.ModelConditionReady) {
			readyCondition = &updated.Status.Conditions[i]
			break
		}
	}
	if readyCondition == nil {
		t.Fatalf("expected ready condition, got %#v", updated.Status.Conditions)
	}
	if readyCondition.Reason != string(expectedReason) {
		t.Fatalf("condition reason = %q, want %q", readyCondition.Reason, expectedReason)
	}
}
