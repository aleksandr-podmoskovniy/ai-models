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
	"sync/atomic"
	"testing"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestMaterializeWithCoordinationPublishesCurrentFromSharedCacheWriter(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "modelcache")
	destination := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef")
	cfg := Coordination{Mode: CoordinationModeShared, HolderID: "pod-a"}
	var runs int32

	result, err := MaterializeWithCoordination(context.Background(), cacheRoot, destination, cfg, func(context.Context) (modelpackports.MaterializeResult, error) {
		atomic.AddInt32(&runs, 1)
		return writeReadyMaterialization(t, destination, "sha256:deadbeef", time.Now().UTC()), nil
	})
	if err != nil {
		t.Fatalf("MaterializeWithCoordination() error = %v", err)
	}
	if got, want := atomic.LoadInt32(&runs), int32(1); got != want {
		t.Fatalf("runner calls = %d, want %d", got, want)
	}
	if got, want := result.ModelPath, CurrentLinkPath(cacheRoot); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(CurrentLinkPath(cacheRoot), "config.json")); err != nil {
		t.Fatalf("expected config through current symlink: %v", err)
	}
}

func TestMaterializeWithCoordinationWaitsForReadyMaterialization(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "modelcache")
	destination := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef")
	cfg := Coordination{Mode: CoordinationModeShared, HolderID: "pod-b"}
	lock := newMaterializationLock(cacheRoot, destination, "pod-a")
	if err := os.MkdirAll(lock.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(lock) error = %v", err)
	}
	if err := writeMaterializationHeartbeat(lock); err != nil {
		t.Fatalf("writeMaterializationHeartbeat() error = %v", err)
	}

	var runs int32
	go func() {
		time.Sleep(100 * time.Millisecond)
		writeReadyMaterialization(t, destination, "sha256:deadbeef", time.Now().UTC())
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := MaterializeWithCoordination(ctx, cacheRoot, destination, cfg, func(context.Context) (modelpackports.MaterializeResult, error) {
		atomic.AddInt32(&runs, 1)
		return modelpackports.MaterializeResult{}, context.Canceled
	})
	if err != nil {
		t.Fatalf("MaterializeWithCoordination() error = %v", err)
	}
	if got, want := atomic.LoadInt32(&runs), int32(0); got != want {
		t.Fatalf("runner calls = %d, want %d", got, want)
	}
	if got, want := result.ModelPath, CurrentLinkPath(cacheRoot); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
}

func TestMaterializeWithCoordinationBreaksStaleLock(t *testing.T) {
	t.Parallel()

	cacheRoot := filepath.Join(t.TempDir(), "modelcache")
	destination := filepath.Join(cacheRoot, StoreDirName, "sha256:deadbeef")
	cfg := Coordination{Mode: CoordinationModeShared, HolderID: "pod-b"}
	lock := newMaterializationLock(cacheRoot, destination, "pod-a")
	if err := os.MkdirAll(lock.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(lock) error = %v", err)
	}
	if err := writeMaterializationHeartbeat(lock); err != nil {
		t.Fatalf("writeMaterializationHeartbeat() error = %v", err)
	}
	staleTime := time.Now().Add(-coordinationLockStaleAfter - time.Minute)
	if err := os.Chtimes(lock.HeartbeatPath, staleTime, staleTime); err != nil {
		t.Fatalf("Chtimes(heartbeat) error = %v", err)
	}

	var runs int32
	result, err := MaterializeWithCoordination(context.Background(), cacheRoot, destination, cfg, func(context.Context) (modelpackports.MaterializeResult, error) {
		atomic.AddInt32(&runs, 1)
		return writeReadyMaterialization(t, destination, "sha256:deadbeef", time.Now().UTC()), nil
	})
	if err != nil {
		t.Fatalf("MaterializeWithCoordination() error = %v", err)
	}
	if got, want := atomic.LoadInt32(&runs), int32(1); got != want {
		t.Fatalf("runner calls = %d, want %d", got, want)
	}
	if got, want := result.ModelPath, CurrentLinkPath(cacheRoot); got != want {
		t.Fatalf("model path = %q, want %q", got, want)
	}
}
