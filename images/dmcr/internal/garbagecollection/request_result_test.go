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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
