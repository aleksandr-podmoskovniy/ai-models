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

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

func TestResolveMaterializeCoordinationReadsSharedCacheMode(t *testing.T) {
	t.Setenv(materializeCoordinationModeEnv, nodecache.CoordinationModeShared)
	t.Setenv(materializeCoordinationHolderEnv, "pod-a")

	cfg, err := resolveMaterializeCoordination("/cache")
	if err != nil {
		t.Fatalf("resolveMaterializeCoordination() error = %v", err)
	}
	if got, want := cfg.Mode, nodecache.CoordinationModeShared; got != want {
		t.Fatalf("mode = %q, want %q", got, want)
	}
	if got, want := cfg.HolderID, "pod-a"; got != want {
		t.Fatalf("holder id = %q, want %q", got, want)
	}
}
