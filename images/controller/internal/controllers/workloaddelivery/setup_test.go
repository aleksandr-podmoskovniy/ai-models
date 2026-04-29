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

package workloaddelivery

import (
	"testing"
)

func TestCoreWorkloadKindsAreStableBuiltInPodTemplateOwners(t *testing.T) {
	t.Parallel()

	got := make([]string, 0, len(coreWorkloadKinds))
	for _, kind := range coreWorkloadKinds {
		got = append(got, kind.kind)
	}
	want := []string{"Deployment", "StatefulSet", "DaemonSet", "CronJob"}
	if len(got) != len(want) {
		t.Fatalf("kind count = %d, want %d: %v", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("kind[%d] = %q, want %q", index, got[index], want[index])
		}
	}
}
