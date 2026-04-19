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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMaintainOnceEvictsMalformedAndIdleEntries(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	currentDestination := filepath.Join(cacheRoot, StoreDirName, "sha256:current")
	oldDestination := filepath.Join(cacheRoot, StoreDirName, "sha256:old")
	brokenDestination := filepath.Join(cacheRoot, StoreDirName, "sha256:broken")
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	currentResult := writeReadyMaterialization(t, currentDestination, "sha256:current", now.Add(-10*time.Minute))
	if err := TouchUsage(currentDestination, now.Add(-1*time.Minute)); err != nil {
		t.Fatalf("TouchUsage(current) error = %v", err)
	}
	if err := UpdateCurrentLink(cacheRoot, currentResult.ModelPath); err != nil {
		t.Fatalf("UpdateCurrentLink() error = %v", err)
	}

	writeReadyMaterialization(t, oldDestination, "sha256:old", now.Add(-48*time.Hour))
	if err := TouchUsage(oldDestination, now.Add(-36*time.Hour)); err != nil {
		t.Fatalf("TouchUsage(old) error = %v", err)
	}

	if err := os.MkdirAll(brokenDestination, 0o755); err != nil {
		t.Fatalf("MkdirAll(broken) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDestination, MarkerFileName), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(broken marker) error = %v", err)
	}

	result, err := MaintainOnce(MaintenanceOptions{
		CacheRoot:         cacheRoot,
		MaxUnusedAge:      24 * time.Hour,
		MaxTotalSizeBytes: 1,
	})
	if err != nil {
		t.Fatalf("MaintainOnce() error = %v", err)
	}
	if got, want := len(result.Plan.Candidates), 2; got != want {
		t.Fatalf("candidate count = %d, want %d", got, want)
	}
	if _, err := os.Stat(currentDestination); err != nil {
		t.Fatalf("expected current entry to stay present: %v", err)
	}
	if _, err := os.Stat(oldDestination); !os.IsNotExist(err) {
		t.Fatalf("expected old entry to be evicted, got err=%v", err)
	}
	if _, err := os.Stat(brokenDestination); !os.IsNotExist(err) {
		t.Fatalf("expected malformed entry to be evicted, got err=%v", err)
	}
}

func TestValidateMaintenanceOptionsRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	cases := []MaintenanceOptions{
		{},
		{CacheRoot: "/cache", MaxTotalSizeBytes: -1},
		{CacheRoot: "/cache", MaxUnusedAge: -1},
		{CacheRoot: "/cache", ScanInterval: -1},
	}
	for _, tc := range cases {
		if err := ValidateMaintenanceOptions(tc); err == nil {
			t.Fatalf("expected validation error for %#v", tc)
		}
	}
}
