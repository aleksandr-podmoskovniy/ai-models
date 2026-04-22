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

import "testing"

func TestBuildReportMarksOnlyPrefixesMissingFromLiveOwnership(t *testing.T) {
	t.Parallel()

	live := newLivePrefixSet()
	live.addRepository("dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/gemma/1111")
	live.addRaw("raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef")

	report := buildReport(
		live,
		[]PrefixInventoryEntry{
			{Prefix: "dmcr/docker/registry/v2/repositories/ai-models/catalog/namespaced/team-a/gemma/1111", ObjectCount: 3},
			{Prefix: "dmcr/docker/registry/v2/repositories/ai-models/catalog/cluster/gemma/2222", ObjectCount: 5},
		},
		[]PrefixInventoryEntry{
			{Prefix: "raw/1111/source-url/.mirror/huggingface/google/gemma/deadbeef", ObjectCount: 2},
			{Prefix: "raw/2222/source-url/.mirror/huggingface/google/gemma/cafebabe", ObjectCount: 7},
		},
	)

	if got, want := len(report.StaleRepositories), 1; got != want {
		t.Fatalf("stale repository count = %d, want %d", got, want)
	}
	if got, want := report.StaleRepositories[0].Prefix, "dmcr/docker/registry/v2/repositories/ai-models/catalog/cluster/gemma/2222"; got != want {
		t.Fatalf("stale repository prefix = %q, want %q", got, want)
	}
	if got, want := len(report.StaleRawPrefixes), 1; got != want {
		t.Fatalf("stale raw prefix count = %d, want %d", got, want)
	}
	if got, want := report.StaleRawPrefixes[0].Prefix, "raw/2222/source-url/.mirror/huggingface/google/gemma/cafebabe"; got != want {
		t.Fatalf("stale raw prefix = %q, want %q", got, want)
	}
}
