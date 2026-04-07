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

package common

import "testing"

func TestEndpointTypes(t *testing.T) {
	t.Parallel()

	if got := EndpointTypes("text-generation"); len(got) != 2 {
		t.Fatalf("unexpected generative endpoint types %#v", got)
	}
	if got := EndpointTypes("embeddings"); len(got) != 1 || got[0] != "OpenAIEmbeddings" {
		t.Fatalf("unexpected embedding endpoint types %#v", got)
	}
	if got := EndpointTypes("unknown"); len(got) != 0 {
		t.Fatalf("unexpected endpoint types %#v", got)
	}
}

func TestGPUWorkingSetAndMinimumLaunch(t *testing.T) {
	t.Parallel()

	workingSet := GPUWorkingSetGiB(32<<30, 0, "", "")
	if got, want := workingSet, int64(40); got != want {
		t.Fatalf("unexpected working set %d", got)
	}

	minimumLaunch := MinimumGPULaunch(120)
	if got, want := minimumLaunch.AcceleratorCount, int64(2); got != want {
		t.Fatalf("unexpected accelerator count %d", got)
	}
	if got, want := minimumLaunch.AcceleratorMemoryGiB, int64(60); got != want {
		t.Fatalf("unexpected accelerator memory %d", got)
	}
}

func TestEstimateParameterCountFromBytes(t *testing.T) {
	t.Parallel()

	if got, want := EstimateParameterCountFromBytes(8<<30, "", "q4_k_m"), int64(17179869184); got != want {
		t.Fatalf("unexpected parameter count %d", got)
	}
}
