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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSchedulePlannerDueAndWaitDuration(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 4, 22, 1, 59, 0, 0, time.UTC)
	planner, err := newSchedulePlanner("0 2 * * *", startedAt)
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}
	if planner.Due(startedAt) {
		t.Fatal("expected planner to stay idle before the next schedule")
	}
	if got, want := planner.WaitDuration(startedAt), time.Minute; got != want {
		t.Fatalf("wait duration = %s, want %s", got, want)
	}
	if !planner.Due(startedAt.Add(time.Minute)) {
		t.Fatal("expected planner to become due at the scheduled minute")
	}
}

func TestEnsureScheduledRequestCreatesOrUpdatesSecret(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	queuedAt := time.Date(2026, 4, 22, 2, 0, 0, 0, time.UTC)
	if err := ensureScheduledRequest(context.Background(), client, "d8-ai-models", queuedAt); err != nil {
		t.Fatalf("ensureScheduledRequest() error = %v", err)
	}

	secret, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := secret.Annotations[RequestQueuedAtAnnotationKey], queuedAt.Format(time.RFC3339Nano); got != want {
		t.Fatalf("queued annotation = %q, want %q", got, want)
	}
}
