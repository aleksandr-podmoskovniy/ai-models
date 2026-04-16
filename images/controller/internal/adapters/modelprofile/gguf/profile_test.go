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

package gguf

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "deepseek-r1-8b-q4_k_m.gguf"), []byte("weights"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	profile, err := Resolve(Input{
		ModelDir: root,
		Task:     "text-generation",
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := profile.Format, "GGUF"; got != want {
		t.Fatalf("unexpected format %q", got)
	}
	if got, want := profile.Family, "deepseek-r1"; got != want {
		t.Fatalf("unexpected family %q", got)
	}
	if got, want := profile.Quantization, "q4_k_m"; got != want {
		t.Fatalf("unexpected quantization %q", got)
	}
	if got, want := profile.ParameterCount, int64(8_000_000_000); got != want {
		t.Fatalf("unexpected parameter count %d", got)
	}
	if got, want := profile.MinimumLaunch.PlacementType, "GPU"; got != want {
		t.Fatalf("unexpected placement type %q", got)
	}
	if len(profile.CompatibleAcceleratorVendors) != 2 {
		t.Fatalf("unexpected compatible accelerator vendors %#v", profile.CompatibleAcceleratorVendors)
	}
	if profile.MinimumLaunch.AcceleratorMemoryGiB <= 0 {
		t.Fatalf("unexpected minimum launch %#v", profile.MinimumLaunch)
	}
	if len(profile.SupportedEndpointTypes) == 0 {
		t.Fatal("expected endpoint types")
	}
}
