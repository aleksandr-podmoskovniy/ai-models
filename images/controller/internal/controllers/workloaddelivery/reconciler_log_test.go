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
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDeploymentReconcilerSuppressesRepeatedAppliedLogForStaleReconcile(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	stale := workload.DeepCopy()
	reconciler, _ := newDeploymentReconciler(t, model, workload, testRegistryReadAuthSecret())

	logs := newTestLoggerBuffer()
	reconciler.logger = logs.logger

	reconcileDeployment(t, reconciler, workload)
	if got, want := logs.count(`"msg":"runtime delivery applied"`), 1; got != want {
		t.Fatalf("applied log count after first reconcile = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}

	reconcileDeployment(t, reconciler, stale)
	if got, want := logs.count(`"msg":"runtime delivery applied"`), 1; got != want {
		t.Fatalf("applied log count after stale reconcile = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}
	if got := logs.count(`"msg":"runtime delivery changed"`); got != 0 {
		t.Fatalf("changed log count after stale reconcile = %d, want 0, logs=%q", got, logs.buffer.String())
	}
}

func TestDeploymentReconcilerSuppressesRepeatedBlockedLog(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "legacy-model-cache"},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testRegistryReadAuthSecret())

	logs := newTestLoggerBuffer()
	reconciler.logger = logs.logger

	reconcileDeployment(t, reconciler, workload)
	if got, want := logs.count(`"msg":"runtime delivery blocked by workload spec"`), 1; got != want {
		t.Fatalf("blocked log count after first reconcile = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}

	var blocked deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &blocked); err != nil {
		t.Fatalf("Get(blocked deployment) error = %v", err)
	}
	reconcileDeployment(t, reconciler, &blocked)
	if got, want := logs.count(`"msg":"runtime delivery blocked by workload spec"`), 1; got != want {
		t.Fatalf("blocked log count after stable reconcile = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}
}

func TestDeploymentReconcilerLogsMeaningfulRuntimeDeliveryChange(t *testing.T) {
	t.Parallel()

	model := readyModel()
	workload := annotatedDeployment(map[string]string{ModelAnnotation: model.Name}, 1, corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	})
	reconciler, kubeClient := newDeploymentReconciler(t, model, workload, testRegistryReadAuthSecret())

	logs := newTestLoggerBuffer()
	reconciler.logger = logs.logger

	reconcileDeployment(t, reconciler, workload)

	var published modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &published); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	published.Status.Artifact.Digest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	published.Status.Artifact.URI = "registry.internal.local/ai-models/catalog/namespaced/team-a/gemma@" + published.Status.Artifact.Digest
	if err := kubeClient.Update(context.Background(), &published); err != nil {
		t.Fatalf("Update(model) error = %v", err)
	}

	var updated deployment
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(workload), &updated); err != nil {
		t.Fatalf("Get(updated deployment) error = %v", err)
	}

	reconcileDeployment(t, reconciler, &updated)

	if got, want := logs.count(`"msg":"runtime delivery applied"`), 1; got != want {
		t.Fatalf("applied log count = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}
	if got, want := logs.count(`"msg":"runtime delivery changed"`), 1; got != want {
		t.Fatalf("changed log count = %d, want %d, logs=%q", got, want, logs.buffer.String())
	}
	if !strings.Contains(logs.buffer.String(), `"previousArtifactDigest":"sha256:d3a98df3d0fff2a2249cf61339492f260122b703621d667259e832681f008d55"`) {
		t.Fatalf("logs do not contain previousArtifactDigest, logs=%q", logs.buffer.String())
	}
	if !strings.Contains(logs.buffer.String(), `"artifactDigest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`) {
		t.Fatalf("logs do not contain new artifactDigest, logs=%q", logs.buffer.String())
	}
	if !strings.Contains(logs.buffer.String(), `"deliveryMode":"`+string(modeldelivery.DeliveryModeSharedDirect)+`"`) {
		t.Fatalf("logs do not contain delivery mode, logs=%q", logs.buffer.String())
	}
}

type testLoggerBuffer struct {
	buffer *bytes.Buffer
	logger *slog.Logger
}

func newTestLoggerBuffer() testLoggerBuffer {
	buffer := &bytes.Buffer{}
	return testLoggerBuffer{
		buffer: buffer,
		logger: slog.New(slog.NewJSONHandler(buffer, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
}

func (b testLoggerBuffer) count(pattern string) int {
	return strings.Count(b.buffer.String(), pattern)
}

func testRegistryReadAuthSecret() client.Object {
	return testkit.NewOCIRegistryWriteAuthSecret(testRegistryNamespace, testRegistryAuthName)
}
