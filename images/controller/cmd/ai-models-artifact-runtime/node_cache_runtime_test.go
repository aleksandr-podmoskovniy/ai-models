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

package main

import (
	"testing"
	"time"
)

func TestParseNodeCacheRuntimeConfigFromEnv(t *testing.T) {
	t.Setenv(nodeCacheRootEnv, "/cache")
	t.Setenv(nodeCacheMaxSizeEnv, "200Gi")
	t.Setenv(nodeCacheMaxUnusedAgeEnv, "48h")
	t.Setenv(nodeCacheScanIntervalEnv, "10m")
	t.Setenv(nodeCacheNodeNameEnv, "node-1")

	config, exitCode, err := parseNodeCacheRuntimeConfig(nil)
	if err != nil {
		t.Fatalf("parseNodeCacheRuntimeConfig() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got, want := config.CacheRoot, "/cache"; got != want {
		t.Fatalf("CacheRoot = %q, want %q", got, want)
	}
	if got, want := config.MaxTotalSize, "200Gi"; got != want {
		t.Fatalf("MaxTotalSize = %q, want %q", got, want)
	}
	if got, want := config.MaxUnusedAge, 48*time.Hour; got != want {
		t.Fatalf("MaxUnusedAge = %s, want %s", got, want)
	}
	if got, want := config.ScanInterval, 10*time.Minute; got != want {
		t.Fatalf("ScanInterval = %s, want %s", got, want)
	}
	if got, want := config.NodeName, "node-1"; got != want {
		t.Fatalf("NodeName = %q, want %q", got, want)
	}
}

func TestParseNodeCacheRuntimeConfigRejectsEmptyRoot(t *testing.T) {
	t.Setenv(nodeCacheRootEnv, "")

	if _, exitCode, err := parseNodeCacheRuntimeConfig(nil); err == nil || exitCode != 2 {
		t.Fatalf("expected exitCode=2 and error, got exitCode=%d err=%v", exitCode, err)
	}
}

func TestParseNodeCacheRuntimeConfigRejectsEmptyNodeName(t *testing.T) {
	t.Setenv(nodeCacheRootEnv, "/cache")
	t.Setenv(nodeCacheNodeNameEnv, "")

	if _, exitCode, err := parseNodeCacheRuntimeConfig(nil); err == nil || exitCode != 2 {
		t.Fatalf("expected exitCode=2 and error, got exitCode=%d err=%v", exitCode, err)
	}
}

func TestParseNodeCacheRuntimeSize(t *testing.T) {
	sizeBytes, err := parseNodeCacheRuntimeSize("1Gi")
	if err != nil {
		t.Fatalf("parseNodeCacheRuntimeSize() error = %v", err)
	}
	if sizeBytes <= 0 {
		t.Fatalf("expected positive size bytes, got %d", sizeBytes)
	}
	if _, err := parseNodeCacheRuntimeSize("not-a-quantity"); err == nil {
		t.Fatal("expected quantity parse error")
	}
}
