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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBuildReportMarksFreshDirectUploadPrefixStaleWhenNoLiveOwnersRemain(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			lastModified: now.Add(-2 * time.Hour),
		},
	)

	report, err := buildReportWithClock(
		context.Background(),
		newFakeDynamicClient(t),
		store,
		"dmcr",
		now,
		cleanupPolicy{},
	)
	if err != nil {
		t.Fatalf("buildReportWithClock() error = %v", err)
	}
	if got, want := len(report.StaleDirectUploadPrefixes), 1; got != want {
		t.Fatalf("stale direct-upload prefix count = %d, want %d", got, want)
	}
	if got, want := report.StaleDirectUploadPrefixes[0].Prefix, "dmcr/_ai_models/direct-upload/objects/session-fresh"; got != want {
		t.Fatalf("stale prefix = %q, want %q", got, want)
	}
}

func TestBuildReportKeepsFreshDirectUploadPrefixAgeBoundedWhileOwnerIsDeleting(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			lastModified: now.Add(-2 * time.Hour),
		},
	)
	deletingModel := directUploadDeletingModel(now)

	report, err := buildReportWithClock(
		context.Background(),
		newFakeDynamicClient(t, deletingModel),
		store,
		"dmcr",
		now,
		cleanupPolicy{},
	)
	if err != nil {
		t.Fatalf("buildReportWithClock() error = %v", err)
	}
	if got := len(report.StaleDirectUploadPrefixes); got != 0 {
		t.Fatalf("stale direct-upload prefix count = %d, want 0 for default age-bounded cleanup", got)
	}
}

func TestBuildReportMarksFreshDirectUploadPrefixStaleWhenDeleteTriggeredPolicyIgnoresDeletingOwner(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStore(
		fakePrefixObject{
			key:          "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			lastModified: now.Add(-2 * time.Hour),
		},
	)
	report, err := buildReportWithClock(
		context.Background(),
		newFakeDynamicClient(t),
		store,
		"dmcr",
		now,
		cleanupPolicy{ignoreDeletingOwners: true},
	)
	if err != nil {
		t.Fatalf("buildReportWithClock() error = %v", err)
	}
	if got, want := len(report.StaleDirectUploadPrefixes), 1; got != want {
		t.Fatalf("stale direct-upload prefix count = %d, want %d", got, want)
	}
}

func directUploadDeletingModel(now time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "ai.deckhouse.io/v1alpha1",
		"kind":       "Model",
		"metadata": map[string]any{
			"name":              "deleting-model",
			"namespace":         "team-a",
			"deletionTimestamp": metav1.NewTime(now).Format(time.RFC3339),
			"annotations": map[string]any{
				legacyCleanupHandleAnnotationKey: `{"kind":"BackendArtifact","backend":{"repositoryMetadataPrefix":"dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/deleting/1111"}}`,
			},
		},
	}}
}
