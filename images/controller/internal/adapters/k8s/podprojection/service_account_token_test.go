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

package podprojection

import "testing"

func TestKubeAPIServiceAccountVolume(t *testing.T) {
	t.Parallel()

	volume := KubeAPIServiceAccountVolume("kube-api")
	if volume.Name != "kube-api" || volume.Projected == nil {
		t.Fatalf("unexpected projected volume %#v", volume)
	}
	if volume.Projected.DefaultMode == nil || *volume.Projected.DefaultMode != 0o444 {
		t.Fatalf("unexpected default mode %#v", volume.Projected.DefaultMode)
	}
	sources := volume.Projected.Sources
	if len(sources) != 3 {
		t.Fatalf("projected sources = %d, want 3", len(sources))
	}
	token := sources[0].ServiceAccountToken
	if token == nil || token.Path != "token" || token.ExpirationSeconds == nil || *token.ExpirationSeconds != 3600 {
		t.Fatalf("unexpected token projection %#v", token)
	}
	configMap := sources[1].ConfigMap
	if configMap == nil || configMap.Name != "kube-root-ca.crt" || len(configMap.Items) != 1 {
		t.Fatalf("unexpected kube-root-ca projection %#v", configMap)
	}
	downwardAPI := sources[2].DownwardAPI
	if downwardAPI == nil || len(downwardAPI.Items) != 1 || downwardAPI.Items[0].Path != "namespace" {
		t.Fatalf("unexpected namespace projection %#v", downwardAPI)
	}
}
