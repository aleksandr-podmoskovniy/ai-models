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
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureDesiredArtifactsPrefetchesOnlyMissingDigests(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	writeReadyMaterialization(t, StorePath(cacheRoot, "sha256:ready"), "sha256:ready", time.Now().UTC())

	artifacts := []DesiredArtifact{
		{ArtifactURI: "oci://example/model-ready@sha256:ready", Digest: "sha256:ready"},
		{ArtifactURI: "oci://example/model-missing@sha256:missing", Digest: "sha256:missing"},
	}

	var gotDigests []string
	err := EnsureDesiredArtifacts(context.Background(), cacheRoot, artifacts, func(_ context.Context, artifact DesiredArtifact, destinationDir string) error {
		gotDigests = append(gotDigests, artifact.Digest)
		writeReadyMaterialization(t, destinationDir, artifact.Digest, time.Now().UTC())
		return nil
	})
	if err != nil {
		t.Fatalf("EnsureDesiredArtifacts() error = %v", err)
	}

	if len(gotDigests) != 1 || gotDigests[0] != "sha256:missing" {
		t.Fatalf("prefetched digests = %v, want [sha256:missing]", gotDigests)
	}
}

func TestRunRuntimeCycleProtectsDesiredDigestsFromEviction(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "cache")
	now := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)

	protectedDir := StorePath(cacheRoot, "sha256:protected")
	evictDir := StorePath(cacheRoot, "sha256:evict")
	writeReadyMaterialization(t, protectedDir, "sha256:protected", now.Add(-48*time.Hour))
	writeReadyMaterialization(t, evictDir, "sha256:evict", now.Add(-48*time.Hour))
	if err := TouchUsage(protectedDir, now.Add(-36*time.Hour)); err != nil {
		t.Fatalf("TouchUsage(protected) error = %v", err)
	}
	if err := TouchUsage(evictDir, now.Add(-36*time.Hour)); err != nil {
		t.Fatalf("TouchUsage(evict) error = %v", err)
	}

	loader := staticDesiredArtifactLoader{artifacts: []DesiredArtifact{{
		ArtifactURI: "oci://example/model-protected@sha256:protected",
		Digest:      "sha256:protected",
	}}}
	err := runRuntimeCycle(context.Background(), RuntimeOptions{
		Maintenance: MaintenanceOptions{
			CacheRoot:         cacheRoot,
			MaxTotalSizeBytes: 1,
			MaxUnusedAge:      24 * time.Hour,
			ScanInterval:      time.Minute,
		},
	}, loader, func(context.Context, DesiredArtifact, string) error { return nil })
	if err != nil {
		t.Fatalf("runRuntimeCycle() error = %v", err)
	}

	marker, err := ReadMarker(protectedDir)
	if err != nil {
		t.Fatalf("expected protected marker to stay readable: %v", err)
	}
	if marker == nil {
		t.Fatal("expected protected digest marker to stay present")
	}
	if _, err := os.Stat(evictDir); !os.IsNotExist(err) {
		t.Fatalf("expected evicted digest to be removed, got err=%v", err)
	}
	if _, err := Scan(cacheRoot); err != nil {
		t.Fatalf("Scan() after runtime cycle error = %v", err)
	}
}

type staticDesiredArtifactLoader struct {
	artifacts []DesiredArtifact
}

func (l staticDesiredArtifactLoader) LoadDesiredArtifacts(context.Context) ([]DesiredArtifact, error) {
	return l.artifacts, nil
}
