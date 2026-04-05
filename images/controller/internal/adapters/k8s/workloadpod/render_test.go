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

package workloadpod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestVolumeMountsIncludeWorkspaceAndRegistryCA(t *testing.T) {
	t.Parallel()

	mounts := VolumeMounts("registry-ca", corev1.VolumeMount{
		Name:      "http-auth",
		MountPath: "/etc/http-auth",
		ReadOnly:  true,
	})

	if len(mounts) != 3 {
		t.Fatalf("unexpected mount count %d", len(mounts))
	}
	if mounts[0].Name != WorkspaceVolumeName || mounts[0].MountPath != WorkspaceMountPath {
		t.Fatalf("unexpected workspace mount %#v", mounts[0])
	}
	if mounts[1].Name != "registry-ca" {
		t.Fatalf("unexpected registry ca mount %#v", mounts[1])
	}
	if mounts[2].Name != "http-auth" {
		t.Fatalf("unexpected extra mount %#v", mounts[2])
	}
}

func TestVolumesIncludeWorkspaceAndRegistryCA(t *testing.T) {
	t.Parallel()

	volumes := Volumes("registry-ca", corev1.Volume{
		Name: "http-auth",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: "projected-http-auth"},
		},
	})

	if len(volumes) != 3 {
		t.Fatalf("unexpected volume count %d", len(volumes))
	}
	if volumes[0].Name != WorkspaceVolumeName || volumes[0].EmptyDir == nil {
		t.Fatalf("unexpected workspace volume %#v", volumes[0])
	}
	if volumes[1].Name != "registry-ca" {
		t.Fatalf("unexpected registry ca volume %#v", volumes[1])
	}
	if volumes[2].Name != "http-auth" || volumes[2].Secret == nil {
		t.Fatalf("unexpected extra volume %#v", volumes[2])
	}
}
