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

package nodecacheruntime

import (
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

func TestDesiredPod(t *testing.T) {
	t.Parallel()

	pod, err := DesiredPod(RuntimeSpec{
		Namespace:           "d8-ai-models",
		NodeName:            "worker-a",
		RuntimeImage:        "runtime:latest",
		ImagePullSecretName: "registry-creds",
		ServiceAccountName:  "ai-models-node-cache-runtime",
		MaxTotalSize:        "200Gi",
		MaxUnusedAge:        "24h",
		ScanInterval:        "5m",
		OCIInsecure:         true,
		OCIAuthSecretName:   "ai-models-dmcr-auth-read",
		OCIRegistryCASecret: "ai-models-dmcr-ca",
	})
	if err != nil {
		t.Fatalf("DesiredPod() error = %v", err)
	}

	if pod.Name != "ai-models-node-cache-runtime-worker-a" {
		t.Fatalf("unexpected Pod name %q", pod.Name)
	}
	if pod.Spec.NodeName != "worker-a" {
		t.Fatalf("unexpected nodeName %q", pod.Spec.NodeName)
	}
	if pod.Spec.ServiceAccountName != "ai-models-node-cache-runtime" {
		t.Fatalf("unexpected service account %q", pod.Spec.ServiceAccountName)
	}
	if len(pod.Spec.ImagePullSecrets) != 1 || pod.Spec.ImagePullSecrets[0].Name != "registry-creds" {
		t.Fatalf("unexpected imagePullSecrets %#v", pod.Spec.ImagePullSecrets)
	}
	if len(pod.Spec.Volumes) != 2 {
		t.Fatalf("unexpected volumes %#v", pod.Spec.Volumes)
	}
	if pod.Spec.Volumes[0].PersistentVolumeClaim == nil {
		t.Fatalf("expected PVC-backed cache root volume, got %#v", pod.Spec.Volumes[0])
	}
	if pod.Spec.Volumes[1].Secret == nil || pod.Spec.Volumes[1].Secret.SecretName != "ai-models-dmcr-ca" {
		t.Fatalf("unexpected registry CA volume %#v", pod.Spec.Volumes[1])
	}
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("unexpected containers %#v", pod.Spec.Containers)
	}

	env := map[string]string{}
	for _, item := range pod.Spec.Containers[0].Env {
		if item.Value != "" {
			env[item.Name] = item.Value
		}
	}
	if env[nodecache.RuntimeCacheRootEnv] != nodecache.RuntimeCacheRootPath {
		t.Fatalf("unexpected cache root env %#v", env)
	}
	if env[RuntimeNodeNameEnv] != "worker-a" {
		t.Fatalf("unexpected node name env %#v", env)
	}
	if env["AI_MODELS_OCI_CA_FILE"] != registryCAFilePath {
		t.Fatalf("unexpected registry CA env %#v", env)
	}
}

func TestDesiredPodOmitsOptionalRegistryCAAndPullSecret(t *testing.T) {
	t.Parallel()

	pod, err := DesiredPod(RuntimeSpec{
		Namespace:          "d8-ai-models",
		NodeName:           "worker-a",
		RuntimeImage:       "runtime:latest",
		ServiceAccountName: "ai-models-node-cache-runtime",
		MaxTotalSize:       "200Gi",
		MaxUnusedAge:       "24h",
		ScanInterval:       "5m",
		OCIAuthSecretName:  "ai-models-dmcr-auth-read",
	})
	if err != nil {
		t.Fatalf("DesiredPod() error = %v", err)
	}

	if len(pod.Spec.ImagePullSecrets) != 0 {
		t.Fatalf("expected no imagePullSecrets, got %#v", pod.Spec.ImagePullSecrets)
	}
	if len(pod.Spec.Volumes) != 1 {
		t.Fatalf("expected only cache-root volume, got %#v", pod.Spec.Volumes)
	}
	for _, item := range pod.Spec.Containers[0].Env {
		if item.Name == "AI_MODELS_OCI_CA_FILE" {
			t.Fatalf("did not expect AI_MODELS_OCI_CA_FILE env, got %#v", pod.Spec.Containers[0].Env)
		}
	}
}
