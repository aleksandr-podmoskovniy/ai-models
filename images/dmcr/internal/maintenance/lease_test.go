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

package maintenance

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/leaseutil"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLeaseGateActivateAndRelease(t *testing.T) {
	client := fake.NewSimpleClientset()
	gate, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-a", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	gate.now = func() time.Time { return now }

	sequence, release, err := gate.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if sequence != "1" {
		t.Fatalf("sequence = %q, want 1", sequence)
	}

	path := filepath.Join(t.TempDir(), "gate.json")
	mirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", path)
	if err != nil {
		t.Fatalf("NewFileMirror() error = %v", err)
	}
	mirror.now = func() time.Time { return now }
	if err := mirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	checker, err := NewFileChecker(path)
	if err != nil {
		t.Fatalf("NewFileChecker() error = %v", err)
	}
	checker.now = func() time.Time { return now }
	active, err := checker.Active(context.Background())
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if !active {
		t.Fatal("expected active maintenance gate")
	}

	if err := release(context.Background()); err != nil {
		t.Fatalf("release() error = %v", err)
	}
	if err := mirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() after release error = %v", err)
	}
	active, err = checker.Active(context.Background())
	if err != nil {
		t.Fatalf("Active() after release error = %v", err)
	}
	if active {
		t.Fatal("expected released maintenance gate to be inactive")
	}
}

func TestFileMirrorTreatsExpiredLeaseAsInactive(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	gate, err := NewLeaseGate(fake.NewSimpleClientset(), "d8-ai-models", "dmcr-gc-maintenance", "pod-a", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	lease := gate.newLease(now.Add(-2 * time.Minute))
	client := fake.NewSimpleClientset(lease)
	path := filepath.Join(t.TempDir(), "gate.json")
	mirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", path)
	if err != nil {
		t.Fatalf("NewFileMirror() error = %v", err)
	}
	mirror.now = func() time.Time { return now }
	if err := mirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	checker, err := NewFileChecker(path)
	if err != nil {
		t.Fatalf("NewFileChecker() error = %v", err)
	}

	active, err := checker.Active(context.Background())
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if active {
		t.Fatal("expected expired maintenance gate to be inactive")
	}

	updated, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-maintenance", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(lease) error = %v", err)
	}
	if leaseutil.Holder(updated) != "pod-a" {
		t.Fatalf("lease holder changed unexpectedly: %q", leaseutil.Holder(updated))
	}
}

func TestLeaseGateTakesOverExpiredLeaseWithMissingTransitions(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	first, err := NewLeaseGate(fake.NewSimpleClientset(), "d8-ai-models", "dmcr-gc-maintenance", "pod-a", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate(first) error = %v", err)
	}
	lease := first.newLease(now.Add(-2 * time.Minute))
	lease.Spec.LeaseTransitions = nil

	client := fake.NewSimpleClientset(lease)
	second, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-b", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate(second) error = %v", err)
	}
	second.now = func() time.Time { return now }

	sequence, _, err := second.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if sequence != "2" {
		t.Fatalf("sequence = %q, want 2", sequence)
	}
	updated, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-maintenance", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(lease) error = %v", err)
	}
	if leaseutil.Holder(updated) != "pod-b" {
		t.Fatalf("lease holder = %q, want pod-b", leaseutil.Holder(updated))
	}
	if updated.Spec.LeaseTransitions == nil || *updated.Spec.LeaseTransitions != 0 {
		t.Fatalf("leaseTransitions = %v, want 0", updated.Spec.LeaseTransitions)
	}
}

func TestLeaseGateDoesNotTreatCreationTimestampOnlyForeignLeaseAsActive(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "dmcr-gc-maintenance",
			Namespace:         "d8-ai-models",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Second)),
			Annotations:       map[string]string{GateSequenceAnnotationKey: "1"},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       leaseutil.StringPtr("pod-a"),
			LeaseDurationSeconds: leaseutil.Int32Ptr(60),
		},
	}
	client := fake.NewSimpleClientset(lease)
	gate, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-b", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	gate.now = func() time.Time { return now }

	sequence, _, err := gate.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	if sequence != "2" {
		t.Fatalf("sequence = %q, want 2", sequence)
	}
	updated, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-maintenance", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(lease) error = %v", err)
	}
	if leaseutil.Holder(updated) != "pod-b" {
		t.Fatalf("lease holder = %q, want pod-b", leaseutil.Holder(updated))
	}
}

func TestFileMirrorTreatsLeaseWithOnlyCreationTimestampAsInactive(t *testing.T) {
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "dmcr-gc-maintenance",
			Namespace:         "d8-ai-models",
			CreationTimestamp: metav1.NewTime(now.Add(-30 * time.Second)),
			Annotations:       map[string]string{GateSequenceAnnotationKey: "1"},
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       leaseutil.StringPtr("pod-a"),
			LeaseDurationSeconds: leaseutil.Int32Ptr(60),
		},
	}
	client := fake.NewSimpleClientset(lease)
	path := filepath.Join(t.TempDir(), "gate.json")
	mirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", path)
	if err != nil {
		t.Fatalf("NewFileMirror() error = %v", err)
	}
	mirror.now = func() time.Time { return now }
	if err := mirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	checker, err := NewFileChecker(path)
	if err != nil {
		t.Fatalf("NewFileChecker() error = %v", err)
	}
	checker.now = func() time.Time { return now }
	active, err := checker.Active(context.Background())
	if err != nil {
		t.Fatalf("Active() error = %v", err)
	}
	if active {
		t.Fatal("expected timestamp-less maintenance gate to be inactive")
	}
}

func TestLeaseGateRefusesActiveGateHeldByOtherIdentity(t *testing.T) {
	client := fake.NewSimpleClientset()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	first, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-a", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate(first) error = %v", err)
	}
	first.now = func() time.Time { return now }
	if _, _, err := first.Activate(context.Background()); err != nil {
		t.Fatalf("Activate(first) error = %v", err)
	}

	second, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-b", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate(second) error = %v", err)
	}
	second.now = func() time.Time { return now.Add(time.Second) }
	if _, _, err := second.Activate(context.Background()); err == nil {
		t.Fatal("expected second active gate activation to fail")
	}
}

func TestFileMirrorsExposeOneClusterGateToMultiplePods(t *testing.T) {
	client := fake.NewSimpleClientset()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	gate, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "executor", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	gate.now = func() time.Time { return now }
	if _, _, err := gate.Activate(context.Background()); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	for _, pod := range []string{"pod-a", "pod-b"} {
		path := filepath.Join(t.TempDir(), pod, "gate.json")
		mirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", path)
		if err != nil {
			t.Fatalf("NewFileMirror(%s) error = %v", pod, err)
		}
		mirror.now = func() time.Time { return now }
		if err := mirror.Sync(context.Background()); err != nil {
			t.Fatalf("Sync(%s) error = %v", pod, err)
		}
		checker, err := NewFileChecker(path)
		if err != nil {
			t.Fatalf("NewFileChecker(%s) error = %v", pod, err)
		}
		checker.now = func() time.Time { return now }
		active, err := checker.Active(context.Background())
		if err != nil {
			t.Fatalf("Active(%s) error = %v", pod, err)
		}
		if !active {
			t.Fatalf("expected %s mirror to observe active gate", pod)
		}
	}
}
