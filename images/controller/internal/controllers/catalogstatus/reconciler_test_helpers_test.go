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

package catalogstatus

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newModelReconciler(
	t *testing.T,
	sourceWorkers publicationports.SourceWorkerRuntime,
	uploadSessions publicationports.UploadSessionRuntime,
	objects ...client.Object,
) (*ModelReconciler, client.Client) {
	return newModelReconcilerWithSink(t, sourceWorkers, uploadSessions, &fakeAuditSink{}, objects...)
}

func newModelReconcilerWithSink(
	t *testing.T,
	sourceWorkers publicationports.SourceWorkerRuntime,
	uploadSessions publicationports.UploadSessionRuntime,
	auditSink auditsink.Sink,
	objects ...client.Object,
) (*ModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ModelReconciler{baseReconciler{
		client:         kubeClient,
		options:        Options{},
		sourceWorkers:  sourceWorkers,
		uploadSessions: uploadSessions,
		auditSink:      auditSink,
	}}, kubeClient
}

func newClusterModelReconciler(
	t *testing.T,
	sourceWorkers publicationports.SourceWorkerRuntime,
	uploadSessions publicationports.UploadSessionRuntime,
	objects ...client.Object,
) (*ClusterModelReconciler, client.Client) {
	return newClusterModelReconcilerWithSink(t, sourceWorkers, uploadSessions, &fakeAuditSink{}, objects...)
}

func newClusterModelReconcilerWithSink(
	t *testing.T,
	sourceWorkers publicationports.SourceWorkerRuntime,
	uploadSessions publicationports.UploadSessionRuntime,
	auditSink auditsink.Sink,
	objects ...client.Object,
) (*ClusterModelReconciler, client.Client) {
	t.Helper()

	scheme := testkit.NewScheme(t)
	kubeClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		objects...,
	)

	return &ClusterModelReconciler{baseReconciler{
		client:         kubeClient,
		options:        Options{},
		sourceWorkers:  sourceWorkers,
		uploadSessions: uploadSessions,
		auditSink:      auditSink,
	}}, kubeClient
}

func testModel() *modelsv1alpha1.Model {
	return testkit.NewModel()
}

func testClusterModel() *modelsv1alpha1.ClusterModel {
	return testkit.NewClusterModel()
}

func testUploadModel() *modelsv1alpha1.Model {
	return testkit.NewUploadModel()
}
