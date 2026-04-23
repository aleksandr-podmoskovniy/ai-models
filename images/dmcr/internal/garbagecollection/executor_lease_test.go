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
	"testing"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExecutorLeaseCreatesLeaseWhenAbsent(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	client := fake.NewSimpleClientset()
	runner := newTestExecutorLeaseRunner(client, now)

	acquired, err := runner.acquireOrRenew(context.Background())
	if err != nil {
		t.Fatalf("acquireOrRenew() error = %v", err)
	}
	if !acquired {
		t.Fatal("acquireOrRenew() = false, want true")
	}

	lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-executor", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get lease error = %v", err)
	}
	assertLeaseHolder(t, lease, "pod-a")
	if lease.Spec.RenewTime == nil || !lease.Spec.RenewTime.Time.Equal(now) {
		t.Fatalf("renewTime = %v, want %s", lease.Spec.RenewTime, now)
	}
	if lease.Spec.LeaseDurationSeconds == nil || *lease.Spec.LeaseDurationSeconds != 30 {
		t.Fatalf("leaseDurationSeconds = %v, want 30", lease.Spec.LeaseDurationSeconds)
	}
}

func TestExecutorLeaseSkipsWorkWhenAnotherHolderLeaseIsLive(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	existing := leaseForTest("d8-ai-models", "dmcr-gc-executor", "pod-b", now.Add(-10*time.Second), 30*time.Second, 0)
	client := fake.NewSimpleClientset(existing)
	runner := newTestExecutorLeaseRunner(client, now)

	called := false
	handled, err := runner.RunIfHolder(context.Background(), func(context.Context) (bool, error) {
		called = true
		return true, nil
	})
	if err != nil {
		t.Fatalf("RunIfHolder() error = %v", err)
	}
	if handled {
		t.Fatal("RunIfHolder() handled = true, want false")
	}
	if called {
		t.Fatal("RunIfHolder() called work for non-holder")
	}

	lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-executor", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get lease error = %v", err)
	}
	assertLeaseHolder(t, lease, "pod-b")
}

func TestExecutorLeaseTakesOverExpiredLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	existing := leaseForTest("d8-ai-models", "dmcr-gc-executor", "pod-b", now.Add(-45*time.Second), 30*time.Second, 2)
	client := fake.NewSimpleClientset(existing)
	runner := newTestExecutorLeaseRunner(client, now)

	acquired, err := runner.acquireOrRenew(context.Background())
	if err != nil {
		t.Fatalf("acquireOrRenew() error = %v", err)
	}
	if !acquired {
		t.Fatal("acquireOrRenew() = false, want true")
	}

	lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-executor", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get lease error = %v", err)
	}
	assertLeaseHolder(t, lease, "pod-a")
	if lease.Spec.LeaseTransitions == nil || *lease.Spec.LeaseTransitions != 3 {
		t.Fatalf("leaseTransitions = %v, want 3", lease.Spec.LeaseTransitions)
	}
}

func TestExecutorLeaseRenewsOwnLease(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC)
	existing := leaseForTest("d8-ai-models", "dmcr-gc-executor", "pod-a", now.Add(-10*time.Second), 30*time.Second, 4)
	client := fake.NewSimpleClientset(existing)
	runner := newTestExecutorLeaseRunner(client, now)

	acquired, err := runner.acquireOrRenew(context.Background())
	if err != nil {
		t.Fatalf("acquireOrRenew() error = %v", err)
	}
	if !acquired {
		t.Fatal("acquireOrRenew() = false, want true")
	}

	lease, err := client.CoordinationV1().Leases("d8-ai-models").Get(context.Background(), "dmcr-gc-executor", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get lease error = %v", err)
	}
	assertLeaseHolder(t, lease, "pod-a")
	if lease.Spec.RenewTime == nil || !lease.Spec.RenewTime.Time.Equal(now) {
		t.Fatalf("renewTime = %v, want %s", lease.Spec.RenewTime, now)
	}
	if lease.Spec.LeaseTransitions == nil || *lease.Spec.LeaseTransitions != 4 {
		t.Fatalf("leaseTransitions = %v, want 4", lease.Spec.LeaseTransitions)
	}
}

func newTestExecutorLeaseRunner(client *fake.Clientset, now time.Time) *executorLeaseRunner {
	return &executorLeaseRunner{
		client:        client,
		namespace:     "d8-ai-models",
		name:          "dmcr-gc-executor",
		identity:      "pod-a",
		duration:      30 * time.Second,
		renewInterval: time.Minute,
		now: func() time.Time {
			return now
		},
	}
}

func leaseForTest(namespace, name, holder string, renewTime time.Time, duration time.Duration, transitions int32) *coordinationv1.Lease {
	durationSeconds := leaseDurationSeconds(duration)
	microTime := metav1.NewMicroTime(renewTime)
	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       stringPtr(holder),
			LeaseDurationSeconds: int32Ptr(durationSeconds),
			AcquireTime:          &microTime,
			RenewTime:            &microTime,
			LeaseTransitions:     int32Ptr(transitions),
		},
	}
}

func assertLeaseHolder(t *testing.T, lease *coordinationv1.Lease, expected string) {
	t.Helper()

	if lease.Spec.HolderIdentity == nil {
		t.Fatalf("holderIdentity is nil, want %q", expected)
	}
	if *lease.Spec.HolderIdentity != expected {
		t.Fatalf("holderIdentity = %q, want %q", *lease.Spec.HolderIdentity, expected)
	}
}
