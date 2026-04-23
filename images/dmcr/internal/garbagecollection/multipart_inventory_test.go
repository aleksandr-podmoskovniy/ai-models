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
)

func TestDiscoverDirectUploadMultipartInventoryReturnsOnlyOldOrTargetedUploads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStoreWithMultipartUploads(nil,
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-stale/data",
			uploadID:    "upload-stale",
			initiatedAt: now.Add(-48 * time.Hour),
			partCount:   5,
		},
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			uploadID:    "upload-fresh",
			initiatedAt: now.Add(-2 * time.Hour),
			partCount:   3,
		},
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-targeted/data",
			uploadID:    "upload-targeted",
			initiatedAt: now.Add(-2 * time.Hour),
			partCount:   7,
		},
	)

	inventory, err := discoverDirectUploadMultipartInventory(
		context.Background(),
		store,
		"dmcr",
		now,
		cleanupPolicy{
			targetDirectUploadMultipartUploads: map[directUploadMultipartTarget]struct{}{
				{
					ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-targeted/data",
					UploadID:  "upload-targeted",
				}: {},
			},
		},
	)
	if err != nil {
		t.Fatalf("discoverDirectUploadMultipartInventory() error = %v", err)
	}

	if got, want := inventory.StoredUploadCount, 3; got != want {
		t.Fatalf("stored upload count = %d, want %d", got, want)
	}
	if got, want := inventory.StoredPartCount, 15; got != want {
		t.Fatalf("stored part count = %d, want %d", got, want)
	}
	if got, want := len(inventory.StaleUploads), 2; got != want {
		t.Fatalf("stale upload count = %d, want %d", got, want)
	}
	if got, want := inventory.StaleUploads[0].UploadID, "upload-stale"; got != want {
		t.Fatalf("stale upload[0] = %q, want %q", got, want)
	}
	if got, want := inventory.StaleUploads[1].UploadID, "upload-targeted"; got != want {
		t.Fatalf("stale upload[1] = %q, want %q", got, want)
	}
}

func TestBuildReportIncludesDirectUploadMultipartResidue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStoreWithMultipartUploads(nil,
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-stale/data",
			uploadID:    "upload-stale",
			initiatedAt: now.Add(-48 * time.Hour),
			partCount:   845,
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

	if got, want := report.StoredDirectUploadMultipartUploadCount, 1; got != want {
		t.Fatalf("stored multipart upload count = %d, want %d", got, want)
	}
	if got, want := report.StoredDirectUploadMultipartPartCount, 845; got != want {
		t.Fatalf("stored multipart part count = %d, want %d", got, want)
	}
	if got, want := len(report.StaleDirectUploadMultipartUploads), 1; got != want {
		t.Fatalf("stale multipart upload count = %d, want %d", got, want)
	}
}

func TestBuildReportMarksFreshMultipartUploadStaleWhenNoLiveOwnersRemain(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStoreWithMultipartUploads(nil,
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-fresh/data",
			uploadID:    "upload-fresh",
			initiatedAt: now.Add(-2 * time.Hour),
			partCount:   9,
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
	if got, want := len(report.StaleDirectUploadMultipartUploads), 1; got != want {
		t.Fatalf("stale multipart upload count = %d, want %d", got, want)
	}
	if got, want := report.StaleDirectUploadMultipartUploads[0].UploadID, "upload-fresh"; got != want {
		t.Fatalf("stale multipart upload = %q, want %q", got, want)
	}
}

func TestDiscoverDirectUploadMultipartInventorySkipsGoneUploads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)
	store := newFakePrefixStoreWithMultipartUploads(nil,
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-gone/data",
			uploadID:    "upload-gone",
			initiatedAt: now.Add(-48 * time.Hour),
			partCount:   12,
		},
		fakeMultipartUpload{
			key:         "dmcr/_ai_models/direct-upload/objects/session-stale/data",
			uploadID:    "upload-stale",
			initiatedAt: now.Add(-48 * time.Hour),
			partCount:   5,
		},
	)
	store.multipartUploadPartErrs[directUploadMultipartTarget{
		ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-gone/data",
		UploadID:  "upload-gone",
	}] = errMultipartUploadGone

	inventory, err := discoverDirectUploadMultipartInventory(
		context.Background(),
		store,
		"dmcr",
		now,
		cleanupPolicy{},
	)
	if err != nil {
		t.Fatalf("discoverDirectUploadMultipartInventory() error = %v", err)
	}
	if got, want := inventory.StoredUploadCount, 2; got != want {
		t.Fatalf("stored upload count = %d, want %d", got, want)
	}
	if got, want := inventory.StoredPartCount, 5; got != want {
		t.Fatalf("stored part count = %d, want %d", got, want)
	}
	if got, want := len(inventory.StaleUploads), 1; got != want {
		t.Fatalf("stale upload count = %d, want %d", got, want)
	}
	if got, want := inventory.StaleUploads[0].UploadID, "upload-stale"; got != want {
		t.Fatalf("stale upload[0] = %q, want %q", got, want)
	}
}

func TestDeleteStalePrefixesAbortsMultipartUploads(t *testing.T) {
	t.Parallel()

	store := newFakePrefixStore()
	report := Report{
		StaleDirectUploadMultipartUploads: []MultipartUploadInventoryEntry{
			{
				Prefix:    "dmcr/_ai_models/direct-upload/objects/session-stale",
				ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-stale/data",
				UploadID:  "upload-stale",
				PartCount: 153,
			},
		},
	}

	if err := deleteStalePrefixes(context.Background(), store, report); err != nil {
		t.Fatalf("deleteStalePrefixes() error = %v", err)
	}

	if got, want := store.abortedMultipartUploads, []directUploadMultipartTarget{{
		ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-stale/data",
		UploadID:  "upload-stale",
	}}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("aborted multipart uploads = %#v, want %#v", got, want)
	}
}
