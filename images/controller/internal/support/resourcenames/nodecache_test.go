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

package resourcenames

import "testing"

func TestNodeCacheRuntimePodName(t *testing.T) {
	t.Parallel()

	name, err := NodeCacheRuntimePodName("worker-1.example.local")
	if err != nil {
		t.Fatalf("NodeCacheRuntimePodName() error = %v", err)
	}
	if got, want := name, "ai-models-node-cache-runtime-worker-1-example-local"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}

func TestNodeCacheRuntimePVCName(t *testing.T) {
	t.Parallel()

	name, err := NodeCacheRuntimePVCName("worker-1.example.local")
	if err != nil {
		t.Fatalf("NodeCacheRuntimePVCName() error = %v", err)
	}
	if got, want := name, "ai-models-node-cache-worker-1-example-local"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
}
