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
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/cleanupjob"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newModelReconciler(t *testing.T, objects ...client.Object) (*ModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, batchv1.AddToScheme)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ModelReconciler{baseReconciler{
		client:  kubeClient,
		scheme:  scheme,
		options: testCleanupOptions(),
	}}, kubeClient
}

func newClusterModelReconciler(t *testing.T, objects ...client.Object) (*ClusterModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t, batchv1.AddToScheme)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ClusterModelReconciler{baseReconciler{
		client:  kubeClient,
		scheme:  scheme,
		options: testCleanupOptions(),
	}}, kubeClient
}

func testCleanupOptions() Options {
	return Options{
		CleanupJob: cleanupjob.Options{
			Namespace:             "d8-ai-models",
			Image:                 "backend:latest",
			OCIRegistrySecretName: "ai-models-publication-registry",
		},
		RequeueAfter: time.Second,
	}
}

func testModel() *modelsv1alpha1.Model {
	return testkit.NewModel()
}

func testClusterModel() *modelsv1alpha1.ClusterModel {
	return testkit.NewClusterModel()
}

func newDeletingModel() *modelsv1alpha1.Model {
	object := testModel()
	now := metav1.Now()
	object.DeletionTimestamp = &now
	object.Finalizers = []string{Finalizer}
	return object
}

func setCleanupHandle(t *testing.T, object metav1.Object, reference string) {
	t.Helper()

	if err := cleanuphandle.SetOnObject(object, cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Artifact: &cleanuphandle.ArtifactSnapshot{
			Kind: modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:  reference,
		},
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: reference,
		},
	}); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}
}

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
