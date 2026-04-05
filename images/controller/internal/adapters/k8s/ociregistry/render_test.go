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

package ociregistry

import "testing"

func TestEnvWithCA(t *testing.T) {
	t.Parallel()

	env := Env(true, "registry-creds", "registry-ca")
	if len(env) != 4 {
		t.Fatalf("expected 4 env vars with CA, got %d", len(env))
	}
}

func TestVolumeMountsAndVolumesWithoutCA(t *testing.T) {
	t.Parallel()

	if mounts := VolumeMounts(""); len(mounts) != 0 {
		t.Fatalf("expected no mounts without ca, got %#v", mounts)
	}
	if volumes := Volumes(""); len(volumes) != 0 {
		t.Fatalf("expected no volumes without ca, got %#v", volumes)
	}
}
