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
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	if got, want := secret.Annotations[phaseAnnotationKey], phaseQueued; got != want {
		t.Fatalf("phase annotation = %q, want %q", got, want)
	}
}

func TestMaybeEnqueueStartupBackfillRequestQueuesWhenStale(t *testing.T) {
	client := fake.NewSimpleClientset()
	startedAt := time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC)
	planner, err := newSchedulePlanner("0 2 * * *", startedAt)
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	checkCalls := 0
	startupBackfillCheckRunner = func(_ context.Context, configPath string) (Report, error) {
		checkCalls++
		if got, want := configPath, "/etc/dmcr/config.yml"; got != want {
			t.Fatalf("configPath = %q, want %q", got, want)
		}
		return Report{
			StaleDirectUploadPrefixes: []PrefixInventoryEntry{
				{Prefix: "dmcr/_ai_models/direct-upload/objects/session-a"},
			},
		}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           "/etc/dmcr/config.yml",
	}
	queuedAt := time.Date(2026, 4, 23, 18, 26, 0, 0, time.UTC)

	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, queuedAt); err != nil {
		t.Fatalf("maybeEnqueueStartupBackfillRequest() error = %v", err)
	}

	secret, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := secret.Annotations[RequestQueuedAtAnnotationKey], queuedAt.Format(time.RFC3339Nano); got != want {
		t.Fatalf("queued annotation = %q, want %q", got, want)
	}
	if got, want := secret.Annotations[phaseAnnotationKey], phaseQueued; got != want {
		t.Fatalf("phase annotation = %q, want %q", got, want)
	}

	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, queuedAt.Add(time.Minute)); err != nil {
		t.Fatalf("second maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	if got, want := checkCalls, 1; got != want {
		t.Fatalf("startup check calls = %d, want %d", got, want)
	}
}

func TestMaybeEnqueueStartupBackfillRequestSkipsEmptyReport(t *testing.T) {
	client := fake.NewSimpleClientset()
	planner, err := newSchedulePlanner("0 2 * * *", time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	startupBackfillCheckRunner = func(context.Context, string) (Report, error) {
		return Report{}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           "/etc/dmcr/config.yml",
	}
	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, time.Now().UTC()); err != nil {
		t.Fatalf("maybeEnqueueStartupBackfillRequest() error = %v", err)
	}

	_, err = client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get(scheduled secret) error = %v, want not found", err)
	}
}

func TestMaybeEnqueueStartupBackfillRequestSkipsWhenRequestExists(t *testing.T) {
	existing := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-existing",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-23T18:20:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(existing.DeepCopy())
	planner, err := newSchedulePlanner("0 2 * * *", time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	checkCalls := 0
	startupBackfillCheckRunner = func(context.Context, string) (Report, error) {
		checkCalls++
		return Report{
			StaleDirectUploadPrefixes: []PrefixInventoryEntry{
				{Prefix: "dmcr/_ai_models/direct-upload/objects/session-a"},
			},
		}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           "/etc/dmcr/config.yml",
	}
	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, time.Now().UTC()); err != nil {
		t.Fatalf("maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	if got, want := checkCalls, 0; got != want {
		t.Fatalf("startup check calls = %d, want %d", got, want)
	}

	_, err = client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get(scheduled secret) error = %v, want not found", err)
	}
}

func TestMaybeEnqueueStartupBackfillRequestIgnoresCompletedRequest(t *testing.T) {
	completed := completedRequestForTest("dmcr-gc-completed", time.Date(2026, 4, 23, 18, 20, 0, 0, time.UTC))
	client := fake.NewSimpleClientset(completed.DeepCopy())
	planner, err := newSchedulePlanner("0 2 * * *", time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	startupBackfillCheckRunner = func(context.Context, string) (Report, error) {
		return Report{
			StaleDirectUploadPrefixes: []PrefixInventoryEntry{
				{Prefix: "dmcr/_ai_models/direct-upload/objects/session-a"},
			},
		}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           "/etc/dmcr/config.yml",
	}
	now := time.Date(2026, 4, 23, 18, 26, 0, 0, time.UTC)
	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, now); err != nil {
		t.Fatalf("maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	if _, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{}); err != nil {
		t.Fatalf("expected scheduled request to be queued despite completed result: %v", err)
	}
}

func TestMaybeEnqueueStartupBackfillRequestRetriesFailedCheck(t *testing.T) {
	client := fake.NewSimpleClientset()
	startedAt := time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC)
	planner, err := newSchedulePlanner("0 2 * * *", startedAt)
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	checkCalls := 0
	startupBackfillCheckRunner = func(context.Context, string) (Report, error) {
		checkCalls++
		if checkCalls == 1 {
			return Report{}, fmt.Errorf("temporary storage inventory failure")
		}
		return Report{
			StaleDirectUploadMultipartUploads: []MultipartUploadInventoryEntry{
				{
					Prefix:    "dmcr/_ai_models/direct-upload/objects/session-a",
					ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-a/data",
					UploadID:  "upload-a",
					PartCount: 2,
				},
			},
		}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           "/etc/dmcr/config.yml",
	}
	firstAttempt := time.Date(2026, 4, 23, 18, 26, 0, 0, time.UTC)

	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, firstAttempt); err != nil {
		t.Fatalf("first maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	_, err = client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Get(scheduled secret) error after failed check = %v, want not found", err)
	}

	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, firstAttempt.Add(30*time.Second)); err != nil {
		t.Fatalf("early retry maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	if got, want := checkCalls, 1; got != want {
		t.Fatalf("startup check calls before retry delay = %d, want %d", got, want)
	}

	retryAt := firstAttempt.Add(startupBackfillRetryDelay)
	if err := maybeEnqueueStartupBackfillRequest(context.Background(), client, options, planner, retryAt); err != nil {
		t.Fatalf("retry maybeEnqueueStartupBackfillRequest() error = %v", err)
	}
	if got, want := checkCalls, 2; got != want {
		t.Fatalf("startup check calls after retry delay = %d, want %d", got, want)
	}

	secret, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := secret.Annotations[RequestQueuedAtAnnotationKey], retryAt.Format(time.RFC3339Nano); got != want {
		t.Fatalf("queued annotation = %q, want %q", got, want)
	}
}
