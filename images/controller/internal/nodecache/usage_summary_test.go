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

package nodecache

import (
	"testing"
	"time"
)

func TestRuntimeUsageSummaryUsesPostEvictionBytesAndReadyDigests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	result := MaintenanceResult{
		Snapshot: Snapshot{Entries: []Entry{
			{Digest: "sha256:ready", DestinationDir: "/cache/ready", Ready: true, SizeBytes: 40},
			{Digest: "sha256:evicted", DestinationDir: "/cache/evicted", Ready: true, SizeBytes: 30},
			{Digest: "sha256:broken", DestinationDir: "/cache/broken", SizeBytes: 10},
		}},
		Plan: EvictionPlan{
			ResidualSizeBytes: 50,
			Candidates: []Candidate{{
				Digest:         "sha256:evicted",
				DestinationDir: "/cache/evicted",
				ReclaimBytes:   30,
			}},
		},
	}

	summary := NewRuntimeUsageSummary("worker-a", 100, result, now)
	if got, want := summary.UsedBytes, int64(50); got != want {
		t.Fatalf("UsedBytes = %d, want %d", got, want)
	}
	if got, want := summary.AvailableBytes, int64(50); got != want {
		t.Fatalf("AvailableBytes = %d, want %d", got, want)
	}
	if got, want := summary.BudgetAvailableBytes, int64(50); got != want {
		t.Fatalf("BudgetAvailableBytes = %d, want %d", got, want)
	}
	if got, want := summary.EntryCount, 2; got != want {
		t.Fatalf("EntryCount = %d, want %d", got, want)
	}
	if got, want := summary.ReadyEntryCount, 1; got != want {
		t.Fatalf("ReadyEntryCount = %d, want %d", got, want)
	}
	if len(summary.ReadyArtifacts) != 1 || summary.ReadyArtifacts[0].Digest != "sha256:ready" {
		t.Fatalf("unexpected ready artifacts %#v", summary.ReadyArtifacts)
	}
	if !summary.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %s, want %s", summary.UpdatedAt, now)
	}
}

func TestRuntimeUsageSummaryAppliesFilesystemFreeSpaceAsEffectiveAvailable(t *testing.T) {
	t.Parallel()

	summary := RuntimeUsageSummary{AvailableBytes: 100, BudgetAvailableBytes: 100}
	summary.ApplyFilesystemAvailableBytes(64)
	if got, want := summary.AvailableBytes, int64(64); got != want {
		t.Fatalf("AvailableBytes = %d, want %d", got, want)
	}
	if got, want := summary.FilesystemAvailableBytes, int64(64); got != want {
		t.Fatalf("FilesystemAvailableBytes = %d, want %d", got, want)
	}
}

func TestMissingSizeBytesSkipsAlreadyReadyDigests(t *testing.T) {
	t.Parallel()

	summary := RuntimeUsageSummary{ReadyArtifacts: []RuntimeUsageArtifact{{
		Digest:    "sha256:cached",
		SizeBytes: 40,
	}}}

	missing, err := MissingSizeBytes(summary, []DesiredArtifact{{
		ArtifactURI: "oci://cached",
		Digest:      "sha256:cached",
		SizeBytes:   40,
	}, {
		ArtifactURI: "oci://new",
		Digest:      "sha256:new",
		SizeBytes:   60,
	}})
	if err != nil {
		t.Fatalf("MissingSizeBytes() error = %v", err)
	}
	if got, want := missing, int64(60); got != want {
		t.Fatalf("MissingSizeBytes() = %d, want %d", got, want)
	}
}
