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

package garbagecollection

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunRequestCycleActivatesAndReleasesMaintenanceGate(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-13T13:40:00Z",
				switchAnnotationKey:          "2026-04-13T00:00:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	options := Options{
		RequestNamespace:        "d8-ai-models",
		RequestLabelSelector:    DefaultRequestLabelSelector(),
		ConfigPath:              filepath.Join(t.TempDir(), "config.yml"),
		GCTimeout:               time.Minute,
		ExecutorIdentity:        "pod-a",
		MaintenanceGateName:     "dmcr-gc-maintenance",
		MaintenanceGateDuration: time.Minute,
		MaintenanceGateDelay:    0,
	}
	if err := os.WriteFile(options.ConfigPath, []byte("storage:\n  sealeds3: {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}

	previousCleanupRunner := cleanupRunner
	cleanupRunner = func(ctx context.Context, _, _ string, _ time.Duration, _ cleanupPolicy) (CleanupResult, error) {
		lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(ctx, "dmcr-gc-maintenance", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Get(maintenance lease) error = %v", err)
		}
		if leaseHolder(lease) != "pod-a" {
			t.Fatalf("maintenance gate holder = %q, want pod-a", leaseHolder(lease))
		}
		return CleanupResult{}, nil
	}
	t.Cleanup(func() {
		cleanupRunner = previousCleanupRunner
	})

	_, err := runRequestCycle(context.Background(), client, options, time.Now)
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-maintenance", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(maintenance lease) after run error = %v", err)
	}
	if leaseHolder(lease) != "" {
		t.Fatalf("maintenance gate holder after release = %q, want empty", leaseHolder(lease))
	}
}

func TestRunRequestCycleWaitsForMaintenanceGateAckQuorum(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-13T13:40:00Z",
				switchAnnotationKey:          "2026-04-13T00:00:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	gateFile := filepath.Join(t.TempDir(), "gate.json")
	options := Options{
		RequestNamespace:              "d8-ai-models",
		RequestLabelSelector:          DefaultRequestLabelSelector(),
		ConfigPath:                    filepath.Join(t.TempDir(), "config.yml"),
		GCTimeout:                     time.Second,
		ExecutorIdentity:              "pod-a",
		MaintenanceGateName:           "dmcr-gc-maintenance",
		MaintenanceGateDuration:       2 * time.Second,
		MaintenanceGateDelay:          2 * time.Second,
		MaintenanceGateFile:           gateFile,
		MaintenanceGateMirrorInterval: 10 * time.Millisecond,
		MaintenanceGateAckQuorum:      1,
		MaintenanceGateAckTTL:         time.Second,
	}
	if err := os.WriteFile(options.ConfigPath, []byte("storage:\n  sealeds3: {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, component := range []string{"dmcr", "direct-upload"} {
		observer, err := maintenance.NewFileAckObserver(gateFile, component, 5*time.Millisecond)
		if err != nil {
			t.Fatalf("NewFileAckObserver(%s) error = %v", component, err)
		}
		observer.Start(ctx)
	}

	cleanupCalled := false
	previousCleanupRunner := cleanupRunner
	cleanupRunner = func(context.Context, string, string, time.Duration, cleanupPolicy) (CleanupResult, error) {
		cleanupCalled = true
		return CleanupResult{}, nil
	}
	t.Cleanup(func() {
		cleanupRunner = previousCleanupRunner
	})

	handled, err := runRequestCycle(context.Background(), client, options, time.Now)
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	if !handled {
		t.Fatal("runRequestCycle() = false, want true")
	}
	if !cleanupCalled {
		t.Fatal("expected cleanup to run after ack quorum")
	}
}

func TestRunRequestCycleSkipsCleanupWhenMaintenanceGateAckQuorumMissing(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-13T13:40:00Z",
				switchAnnotationKey:          "2026-04-13T00:00:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	options := Options{
		RequestNamespace:              "d8-ai-models",
		RequestLabelSelector:          DefaultRequestLabelSelector(),
		ConfigPath:                    filepath.Join(t.TempDir(), "config.yml"),
		GCTimeout:                     time.Second,
		ExecutorIdentity:              "pod-a",
		MaintenanceGateName:           "dmcr-gc-maintenance",
		MaintenanceGateDuration:       2 * time.Second,
		MaintenanceGateDelay:          20 * time.Millisecond,
		MaintenanceGateFile:           filepath.Join(t.TempDir(), "gate.json"),
		MaintenanceGateMirrorInterval: 5 * time.Millisecond,
		MaintenanceGateAckQuorum:      1,
		MaintenanceGateAckTTL:         time.Second,
	}
	if err := os.WriteFile(options.ConfigPath, []byte("storage:\n  sealeds3: {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}

	cleanupCalled := false
	previousCleanupRunner := cleanupRunner
	cleanupRunner = func(context.Context, string, string, time.Duration, cleanupPolicy) (CleanupResult, error) {
		cleanupCalled = true
		return CleanupResult{}, nil
	}
	t.Cleanup(func() {
		cleanupRunner = previousCleanupRunner
	})

	handled, err := runRequestCycle(context.Background(), client, options, time.Now)
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	if handled {
		t.Fatal("runRequestCycle() = true, want false while ack quorum is missing")
	}
	if cleanupCalled {
		t.Fatal("cleanup ran before ack quorum")
	}
	if _, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-request-1", metav1.GetOptions{}); err != nil {
		t.Fatalf("active request secret should stay for retry: %v", err)
	}
}
