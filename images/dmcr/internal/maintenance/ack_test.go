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

	"k8s.io/client-go/kubernetes/fake"
)

func TestAckMirrorPublishesQuorumOnlyAfterAllRuntimeAcks(t *testing.T) {
	client := fake.NewSimpleClientset()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	gate, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "executor", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	gate.now = func() time.Time { return now }
	sequence, _, err := gate.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	gatePath := filepath.Join(t.TempDir(), "gate.json")
	fileMirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", gatePath)
	if err != nil {
		t.Fatalf("NewFileMirror() error = %v", err)
	}
	fileMirror.now = func() time.Time { return now }
	if err := fileMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(gate) error = %v", err)
	}

	ackMirror, err := NewAckMirror(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-a", gatePath, RuntimeAckComponents, 5*time.Second)
	if err != nil {
		t.Fatalf("NewAckMirror() error = %v", err)
	}
	ackMirror.now = func() time.Time { return now }
	if err := ackMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(ack without files) error = %v", err)
	}
	count, err := AckQuorumReady(context.Background(), client, "d8-ai-models", "dmcr-gc-maintenance", sequence, 1, now)
	if err != nil {
		t.Fatalf("AckQuorumReady() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ack count without runtime acks = %d, want 0", count)
	}

	for _, component := range RuntimeAckComponents {
		observer, err := NewFileAckObserver(gatePath, component, time.Second)
		if err != nil {
			t.Fatalf("NewFileAckObserver(%s) error = %v", component, err)
		}
		observer.now = func() time.Time { return now }
		if err := observer.Sync(); err != nil {
			t.Fatalf("Sync(observer %s) error = %v", component, err)
		}
	}
	if err := ackMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(ack with files) error = %v", err)
	}
	count, err = AckQuorumReady(context.Background(), client, "d8-ai-models", "dmcr-gc-maintenance", sequence, 1, now)
	if err != nil {
		t.Fatalf("AckQuorumReady() after acks error = %v", err)
	}
	if count != 1 {
		t.Fatalf("ack count after runtime acks = %d, want 1", count)
	}
}

func TestAckMirrorRejectsStaleSequence(t *testing.T) {
	client := fake.NewSimpleClientset()
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	gate, err := NewLeaseGate(client, "d8-ai-models", "dmcr-gc-maintenance", "executor", time.Minute)
	if err != nil {
		t.Fatalf("NewLeaseGate() error = %v", err)
	}
	gate.now = func() time.Time { return now }
	oldSequence, _, err := gate.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate(old) error = %v", err)
	}

	gatePath := filepath.Join(t.TempDir(), "gate.json")
	fileMirror, err := NewFileMirror(client, "d8-ai-models", "dmcr-gc-maintenance", gatePath)
	if err != nil {
		t.Fatalf("NewFileMirror() error = %v", err)
	}
	fileMirror.now = func() time.Time { return now }
	if err := fileMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(old gate) error = %v", err)
	}
	for _, component := range RuntimeAckComponents {
		observer, err := NewFileAckObserver(gatePath, component, time.Second)
		if err != nil {
			t.Fatalf("NewFileAckObserver(%s) error = %v", component, err)
		}
		observer.now = func() time.Time { return now }
		if err := observer.Sync(); err != nil {
			t.Fatalf("Sync(observer %s) error = %v", component, err)
		}
	}

	now = now.Add(time.Second)
	gate.now = func() time.Time { return now }
	newSequence, _, err := gate.Activate(context.Background())
	if err != nil {
		t.Fatalf("Activate(new) error = %v", err)
	}
	if newSequence == oldSequence {
		t.Fatalf("new sequence = old sequence %q", newSequence)
	}
	fileMirror.now = func() time.Time { return now }
	if err := fileMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(new gate) error = %v", err)
	}

	ackMirror, err := NewAckMirror(client, "d8-ai-models", "dmcr-gc-maintenance", "pod-a", gatePath, RuntimeAckComponents, 5*time.Second)
	if err != nil {
		t.Fatalf("NewAckMirror() error = %v", err)
	}
	ackMirror.now = func() time.Time { return now }
	if err := ackMirror.Sync(context.Background()); err != nil {
		t.Fatalf("Sync(stale ack files) error = %v", err)
	}
	count, err := AckQuorumReady(context.Background(), client, "d8-ai-models", "dmcr-gc-maintenance", newSequence, 1, now)
	if err != nil {
		t.Fatalf("AckQuorumReady() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("ack count for stale sequence = %d, want 0", count)
	}
}
