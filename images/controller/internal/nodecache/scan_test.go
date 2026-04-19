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

func TestScanReportsReadyCurrentAndMalformedEntries(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	readyDestination := filepath.Join(cacheRoot, StoreDirName, "sha256:ready")
	readyAt := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	result := writeReadyMaterialization(t, readyDestination, "sha256:ready", readyAt)
	if err := TouchUsage(readyDestination, readyAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("TouchUsage() error = %v", err)
	}
	if err := UpdateCurrentLink(cacheRoot, result.ModelPath); err != nil {
		t.Fatalf("UpdateCurrentLink() error = %v", err)
	}

	malformedDestination := filepath.Join(cacheRoot, StoreDirName, "sha256:broken")
	if err := os.MkdirAll(filepath.Join(malformedDestination, "tmp"), 0o755); err != nil {
		t.Fatalf("MkdirAll(malformed) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(malformedDestination, MarkerFileName), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(broken marker) error = %v", err)
	}

	snapshot, err := Scan(cacheRoot)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if got, want := len(snapshot.Entries), 2; got != want {
		t.Fatalf("entry count = %d, want %d", got, want)
	}
	if snapshot.TotalSizeBytes == 0 {
		t.Fatalf("expected non-zero cache size")
	}

	readyEntry := snapshot.Entries[1]
	if readyEntry.Digest != "sha256:ready" {
		readyEntry = snapshot.Entries[0]
	}
	if !readyEntry.Ready || !readyEntry.Current {
		t.Fatalf("expected ready current entry, got %#v", readyEntry)
	}
	if readyEntry.LastUsedAt.IsZero() {
		t.Fatalf("expected last-used timestamp in ready entry")
	}

	brokenEntry := snapshot.Entries[0]
	if brokenEntry.Digest == "sha256:ready" {
		brokenEntry = snapshot.Entries[1]
	}
	if brokenEntry.Ready || brokenEntry.Failure == "" {
		t.Fatalf("expected malformed entry, got %#v", brokenEntry)
	}
}
