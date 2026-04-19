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
	"path/filepath"
	"testing"
	"time"
)

func TestPlanEvictionPrioritizesMalformedAndIdleEntries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		TotalSizeBytes: 120,
		Entries: []Entry{
			{
				Digest:         "sha256:current",
				DestinationDir: filepath.Join("/cache", StoreDirName, "sha256:current"),
				SizeBytes:      50,
				Ready:          true,
				Current:        true,
				LastUsedAt:     now.Add(-1 * time.Minute),
			},
			{
				Digest:         "sha256:old",
				DestinationDir: filepath.Join("/cache", StoreDirName, "sha256:old"),
				SizeBytes:      30,
				Ready:          true,
				LastUsedAt:     now.Add(-10 * time.Hour),
			},
			{
				Digest:         "sha256:broken",
				DestinationDir: filepath.Join("/cache", StoreDirName, "sha256:broken"),
				SizeBytes:      40,
				Failure:        "broken marker",
			},
		},
	}

	plan := PlanEviction(snapshot, PlanInput{
		Now:               now,
		MaxUnusedAge:      time.Hour,
		MaxTotalSizeBytes: 60,
	})
	if got, want := len(plan.Candidates), 2; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
	if got, want := plan.Candidates[0].Reason, CandidateReasonMalformed; got != want {
		t.Fatalf("first reason = %q, want %q", got, want)
	}
	if got, want := plan.Candidates[1].Reason, CandidateReasonUnusedAge; got != want {
		t.Fatalf("second reason = %q, want %q", got, want)
	}
	if got, want := plan.ResidualSizeBytes, int64(50); got != want {
		t.Fatalf("residual size = %d, want %d", got, want)
	}
}

func TestPlanEvictionSkipsProtectedDigests(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	snapshot := Snapshot{
		TotalSizeBytes: 120,
		Entries: []Entry{
			{
				Digest:         "sha256:protected",
				DestinationDir: filepath.Join("/cache", StoreDirName, "sha256:protected"),
				SizeBytes:      80,
				Ready:          true,
				LastUsedAt:     now.Add(-10 * time.Hour),
			},
			{
				Digest:         "sha256:old",
				DestinationDir: filepath.Join("/cache", StoreDirName, "sha256:old"),
				SizeBytes:      40,
				Ready:          true,
				LastUsedAt:     now.Add(-9 * time.Hour),
			},
		},
	}

	plan := PlanEviction(snapshot, PlanInput{
		Now:               now,
		MaxUnusedAge:      time.Hour,
		MaxTotalSizeBytes: 10,
		ProtectedDigests:  []string{"sha256:protected"},
	})
	if got, want := len(plan.Candidates), 1; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
	if got, want := plan.Candidates[0].Digest, "sha256:old"; got != want {
		t.Fatalf("candidate digest = %q, want %q", got, want)
	}
}
