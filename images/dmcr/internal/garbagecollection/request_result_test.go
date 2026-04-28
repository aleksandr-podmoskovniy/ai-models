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
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestPruneExpiredCompletedRequestsDeletesOnlyExpiredResults(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
	expired := completedRequestForTest("dmcr-gc-expired", now.Add(-2*time.Hour))
	fresh := completedRequestForTest("dmcr-gc-fresh", now.Add(-5*time.Minute))
	queued := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-queued",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
				phaseAnnotationKey:           phaseQueued,
			},
		},
	}
	client := fake.NewSimpleClientset(expired.DeepCopy(), fresh.DeepCopy(), queued.DeepCopy())

	kept, err := pruneExpiredCompletedRequests(
		context.Background(),
		client,
		"d8-ai-models",
		[]corev1.Secret{expired, fresh, queued},
		now,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("pruneExpiredCompletedRequests() error = %v", err)
	}
	if got, want := secretNames(kept), []string{"dmcr-gc-fresh", "dmcr-gc-queued"}; !stringSlicesEqual(got, want) {
		t.Fatalf("kept secrets = %#v, want %#v", got, want)
	}
	if _, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-expired", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expired secret Get error = %v, want not found", err)
	}
	if _, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-fresh", metav1.GetOptions{}); err != nil {
		t.Fatalf("fresh secret should stay observable: %v", err)
	}
}

func TestCompletedRequestWithMalformedTimestampExpiresFailOpen(t *testing.T) {
	t.Parallel()

	secret := completedRequestForTest("dmcr-gc-broken", time.Time{})
	secret.Annotations[completedAtAnnotationKey] = "broken"
	if !completedRequestExpired(secret, time.Now().UTC(), time.Hour) {
		t.Fatal("completedRequestExpired() = false, want true for malformed timestamp")
	}
}

func TestBoundedResultRegistryOutputTruncatesLargeOutput(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("x", maxResultRegistryOutputBytes+10)
	got := boundedResultRegistryOutput(raw)
	if len(got) <= maxResultRegistryOutputBytes {
		t.Fatalf("bounded output length = %d, want truncation suffix", len(got))
	}
	if !strings.Contains(got, "...truncated...") {
		t.Fatalf("bounded output = %q, want truncation marker", got)
	}
}

func TestRequestResultIncludesDeletedRegistryBlobCount(t *testing.T) {
	t.Parallel()

	result := requestResult(CleanupResult{
		DeletedRegistryBlobCount: 2,
	}, time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC), []string{"dmcr-gc-a"})

	if result.DeletedRegistryBlobCount != 2 {
		t.Fatalf("deleted registry blob count = %d, want 2", result.DeletedRegistryBlobCount)
	}
}

func TestCountDeletedRegistryBlobs(t *testing.T) {
	t.Parallel()

	output := strings.Join([]string{
		"blob eligible for deletion: sha256:a",
		`time="2026-04-27T20:21:55Z" level=info msg="Deleting blob: /docker/registry/v2/blobs/sha256/aa"`,
		"manifest eligible for deletion: sha256:b",
		`time="2026-04-27T20:21:56Z" level=info msg="Deleting blob: /docker/registry/v2/blobs/sha256/bb"`,
	}, "\n")

	if got := countDeletedRegistryBlobs(output); got != 2 {
		t.Fatalf("deleted registry blob count = %d, want 2", got)
	}
}

func TestMarkRequestsCompletedLeavesFailedUpdatesReplayable(t *testing.T) {
	now := time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC)
	first := activeRequestForResultTest("dmcr-gc-a", now.Add(-time.Minute))
	second := activeRequestForResultTest("dmcr-gc-b", now.Add(-time.Minute))
	client := fake.NewSimpleClientset(first.DeepCopy(), second.DeepCopy())

	failSecondUpdateOnce := true
	client.Fake.PrependReactor("update", "secrets", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updated := action.(k8stesting.UpdateAction).GetObject().(*corev1.Secret)
		if updated.Name != "dmcr-gc-b" || !failSecondUpdateOnce {
			return false, nil, nil
		}
		failSecondUpdateOnce = false
		return true, nil, errors.New("temporary update failure")
	})

	result := CleanupResult{RegistryOutput: "gc-ok"}
	err := markRequestsCompleted(context.Background(), client, "d8-ai-models", []corev1.Secret{first, second}, result, now)
	if err == nil {
		t.Fatal("markRequestsCompleted() error = nil, want update failure")
	}

	firstAfterFailure := getRequestForTest(t, client, "dmcr-gc-a")
	assertCompletedRequestForTest(t, firstAfterFailure)
	secondAfterFailure := getRequestForTest(t, client, "dmcr-gc-b")
	if !shouldRunGarbageCollection(secondAfterFailure) {
		t.Fatalf("failed request should remain replayable active request: %#v", secondAfterFailure.Annotations)
	}
	if secondAfterFailure.Data[resultDataKey] != nil {
		t.Fatalf("failed request unexpectedly has result data: %#v", secondAfterFailure.Data)
	}

	activeAfterFailure := activeRequestSecrets([]corev1.Secret{firstAfterFailure, secondAfterFailure})
	if got, want := secretNames(activeAfterFailure), []string{"dmcr-gc-b"}; !stringSlicesEqual(got, want) {
		t.Fatalf("active requests after partial completion = %#v, want %#v", got, want)
	}
	if err := markRequestsCompleted(context.Background(), client, "d8-ai-models", activeAfterFailure, result, now.Add(time.Second)); err != nil {
		t.Fatalf("replay markRequestsCompleted() error = %v", err)
	}
	assertCompletedRequestForTest(t, getRequestForTest(t, client, "dmcr-gc-b"))
}

func activeRequestForResultTest(name string, queuedAt time.Time) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: queuedAt.Format(time.RFC3339Nano),
				switchAnnotationKey:          queuedAt.Add(time.Second).Format(time.RFC3339Nano),
				phaseAnnotationKey:           phaseArmed,
			},
		},
		Data: map[string][]byte{
			directUploadTokenDataKey: []byte("token-must-not-survive-result"),
		},
	}
}

func getRequestForTest(t *testing.T, client *fake.Clientset, name string) corev1.Secret {
	t.Helper()

	secret, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(%s) error = %v", name, err)
	}
	return *secret
}

func assertCompletedRequestForTest(t *testing.T, secret corev1.Secret) {
	t.Helper()

	if got := secret.Annotations[phaseAnnotationKey]; got != phaseDone {
		t.Fatalf("%s phase = %q, want %q", secret.Name, got, phaseDone)
	}
	if secret.Annotations[switchAnnotationKey] != "" {
		t.Fatalf("%s switch annotation survived completion: %#v", secret.Name, secret.Annotations)
	}
	if secret.Annotations[completedAtAnnotationKey] == "" {
		t.Fatalf("%s missing completed-at annotation: %#v", secret.Name, secret.Annotations)
	}
	if _, found := secret.Data[directUploadTokenDataKey]; found {
		t.Fatalf("%s direct-upload token survived completion: %#v", secret.Name, secret.Data)
	}
	if secret.Data[resultDataKey] == nil {
		t.Fatalf("%s missing result data: %#v", secret.Name, secret.Data)
	}
}

func completedRequestForTest(name string, completedAt time.Time) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: completedAt.Add(-time.Minute).Format(time.RFC3339Nano),
				completedAtAnnotationKey:     completedAt.Format(time.RFC3339Nano),
				phaseAnnotationKey:           phaseDone,
			},
		},
	}
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
