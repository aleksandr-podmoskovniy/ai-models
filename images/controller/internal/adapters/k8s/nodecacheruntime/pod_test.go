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
	corev1 "k8s.io/api/core/v1"
)

func TestDesiredPod(t *testing.T) {
	t.Parallel()

	pod, err := DesiredPod(RuntimeSpec{
		Namespace:              "d8-ai-models",
		NodeName:               "worker-a",
		RuntimeImage:           "runtime:latest",
		CSIRegistrarImage:      "registrar:latest",
		ImagePullSecretName:    "registry-creds",
		ServiceAccountName:     "ai-models-node-cache-runtime",
		MaxTotalSize:           "200Gi",
		MaxUnusedAge:           "24h",
		ScanInterval:           "5m",
		OCIInsecure:            true,
		OCIAuthSecretName:      "ai-models-dmcr-auth-read",
		DeliveryAuthSecretName: "ai-models-dmcr-auth",
		OCIRegistryCASecret:    "ai-models-dmcr-ca",
	})
	if err != nil {
		t.Fatalf("DesiredPod() error = %v", err)
	}

	if pod.Name != "ai-models-node-cache-runtime-worker-a" {
		t.Fatalf("unexpected Pod name %q", pod.Name)
	}
	if pod.Spec.NodeName != "" {
		t.Fatalf("runtime Pod must be scheduled through node affinity, got direct nodeName %q", pod.Spec.NodeName)
	}
	if got := requiredHostnameAffinityValue(pod); got != "worker-a" {
		t.Fatalf("unexpected required hostname affinity %q", got)
	}
	if pod.Spec.ServiceAccountName != "ai-models-node-cache-runtime" {
		t.Fatalf("unexpected service account %q", pod.Spec.ServiceAccountName)
	}
	if pod.Spec.AutomountServiceAccountToken == nil || !*pod.Spec.AutomountServiceAccountToken {
		t.Fatalf("expected explicit service account token mount for node-cache runtime, got %#v", pod.Spec.AutomountServiceAccountToken)
	}
	if len(pod.Spec.ImagePullSecrets) != 1 || pod.Spec.ImagePullSecrets[0].Name != "registry-creds" {
		t.Fatalf("unexpected imagePullSecrets %#v", pod.Spec.ImagePullSecrets)
	}
	if len(pod.Spec.Volumes) != 6 {
		t.Fatalf("unexpected volumes %#v", pod.Spec.Volumes)
	}
	if pod.Spec.Volumes[0].PersistentVolumeClaim == nil {
		t.Fatalf("expected PVC-backed cache root volume, got %#v", pod.Spec.Volumes[0])
	}
	if volumeByName(pod.Spec.Volumes, csiPluginVolumeName).HostPath.Path != nodecache.CSIKubeletPluginDir {
		t.Fatalf("unexpected CSI plugin hostPath %#v", volumeByName(pod.Spec.Volumes, csiPluginVolumeName))
	}
	if volumeByName(pod.Spec.Volumes, csiRegistryVolumeName).HostPath.Path != nodecache.CSIRegistrationDirectory {
		t.Fatalf("unexpected CSI registry hostPath %#v", volumeByName(pod.Spec.Volumes, csiRegistryVolumeName))
	}
	if volumeByName(pod.Spec.Volumes, registryCASecretVolume).Secret.SecretName != "ai-models-dmcr-ca" {
		t.Fatalf("unexpected registry CA volume %#v", volumeByName(pod.Spec.Volumes, registryCASecretVolume))
	}
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("unexpected containers %#v", pod.Spec.Containers)
	}

	runtime := pod.Spec.Containers[0]
	if runtime.SecurityContext == nil || runtime.SecurityContext.Privileged == nil || !*runtime.SecurityContext.Privileged {
		t.Fatalf("expected privileged runtime container security context, got %#v", runtime.SecurityContext)
	}
	if got, want := runtime.Args, []string{"--csi-endpoint=" + nodecache.CSIContainerSocketPath}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("unexpected runtime args %#v", got)
	}
	if mountByName(runtime.VolumeMounts, kubeletVolumeName).MountPropagation == nil ||
		*mountByName(runtime.VolumeMounts, kubeletVolumeName).MountPropagation != corev1.MountPropagationBidirectional {
		t.Fatalf("expected bidirectional kubelet mount, got %#v", mountByName(runtime.VolumeMounts, kubeletVolumeName))
	}
	env := map[string]string{}
	for _, item := range runtime.Env {
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
	if got, want := envFieldPathByName(runtime.Env, RuntimePodNameEnv), "metadata.name"; got != want {
		t.Fatalf("runtime pod name env fieldPath = %q, want %q", got, want)
	}
	if got, want := envFieldPathByName(runtime.Env, RuntimePodNamespaceEnv), "metadata.namespace"; got != want {
		t.Fatalf("runtime pod namespace env fieldPath = %q, want %q", got, want)
	}
	if env["AI_MODELS_OCI_CA_FILE"] != registryCAFilePath {
		t.Fatalf("unexpected registry CA env %#v", env)
	}
	if got, want := envSecretKeyByName(runtime.Env, "AI_MODELS_DELIVERY_AUTH_KEY"), "ai-models-dmcr-auth/salt"; got != want {
		t.Fatalf("delivery auth env secret = %q, want %q", got, want)
	}

	registrar := pod.Spec.Containers[1]
	if registrar.Name != RegistrarContainerName || registrar.Image != "registrar:latest" {
		t.Fatalf("unexpected registrar container %#v", registrar)
	}
	if registrar.SecurityContext == nil ||
		registrar.SecurityContext.Capabilities == nil ||
		len(registrar.SecurityContext.Capabilities.Drop) != 1 ||
		registrar.SecurityContext.Capabilities.Drop[0] != "ALL" {
		t.Fatalf("expected registrar to drop all capabilities, got %#v", registrar.SecurityContext)
	}
	if got, want := registrar.Env[0].Value, nodecache.CSIContainerSocketPath; got != want {
		t.Fatalf("registrar CSI endpoint = %q, want %q", got, want)
	}
	if got, want := registrar.Env[1].Value, nodecache.CSIKubeletSocketPath; got != want {
		t.Fatalf("registrar kubelet socket = %q, want %q", got, want)
	}
}

func TestDesiredPodOmitsOptionalRegistryCAAndPullSecret(t *testing.T) {
	t.Parallel()

	pod, err := DesiredPod(RuntimeSpec{
		Namespace:              "d8-ai-models",
		NodeName:               "worker-a",
		RuntimeImage:           "runtime:latest",
		CSIRegistrarImage:      "registrar:latest",
		ServiceAccountName:     "ai-models-node-cache-runtime",
		MaxTotalSize:           "200Gi",
		MaxUnusedAge:           "24h",
		ScanInterval:           "5m",
		OCIAuthSecretName:      "ai-models-dmcr-auth-read",
		DeliveryAuthSecretName: "ai-models-dmcr-auth",
	})
	if err != nil {
		t.Fatalf("DesiredPod() error = %v", err)
	}

	if len(pod.Spec.ImagePullSecrets) != 0 {
		t.Fatalf("expected no imagePullSecrets, got %#v", pod.Spec.ImagePullSecrets)
	}
	if volumeByName(pod.Spec.Volumes, registryCASecretVolume).Name != "" {
		t.Fatalf("did not expect registry CA volume, got %#v", pod.Spec.Volumes)
	}
	for _, item := range pod.Spec.Containers[0].Env {
		if item.Name == "AI_MODELS_OCI_CA_FILE" {
			t.Fatalf("did not expect AI_MODELS_OCI_CA_FILE env, got %#v", pod.Spec.Containers[0].Env)
		}
	}
}

func TestDesiredPodUsesNodeHostnameLabelForScheduling(t *testing.T) {
	t.Parallel()

	pod, err := DesiredPod(RuntimeSpec{
		Namespace:              "d8-ai-models",
		NodeName:               "node-object-a",
		NodeHostname:           "worker-a.example.test",
		RuntimeImage:           "runtime:latest",
		CSIRegistrarImage:      "registrar:latest",
		ServiceAccountName:     "ai-models-node-cache-runtime",
		MaxTotalSize:           "200Gi",
		MaxUnusedAge:           "24h",
		ScanInterval:           "5m",
		OCIAuthSecretName:      "ai-models-dmcr-auth-read",
		DeliveryAuthSecretName: "ai-models-dmcr-auth",
	})
	if err != nil {
		t.Fatalf("DesiredPod() error = %v", err)
	}
	if got := requiredHostnameAffinityValue(pod); got != "worker-a.example.test" {
		t.Fatalf("unexpected required hostname affinity %q", got)
	}
	if got := envByName(pod.Spec.Containers[0].Env, RuntimeNodeNameEnv); got != "node-object-a" {
		t.Fatalf("runtime node identity env = %q, want node object name", got)
	}
}

func volumeByName(volumes []corev1.Volume, name string) corev1.Volume {
	for _, volume := range volumes {
		if volume.Name == name {
			return volume
		}
	}
	return corev1.Volume{}
}

func mountByName(mounts []corev1.VolumeMount, name string) corev1.VolumeMount {
	for _, mount := range mounts {
		if mount.Name == name {
			return mount
		}
	}
	return corev1.VolumeMount{}
}

func requiredHostnameAffinityValue(pod *corev1.Pod) string {
	if pod == nil ||
		pod.Spec.Affinity == nil ||
		pod.Spec.Affinity.NodeAffinity == nil ||
		pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return ""
	}
	for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
		for _, expr := range term.MatchExpressions {
			if expr.Key == corev1.LabelHostname && expr.Operator == corev1.NodeSelectorOpIn && len(expr.Values) == 1 {
				return expr.Values[0]
			}
		}
	}
	return ""
}

func envByName(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}

func envFieldPathByName(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name && item.ValueFrom != nil && item.ValueFrom.FieldRef != nil {
			return item.ValueFrom.FieldRef.FieldPath
		}
	}
	return ""
}

func envSecretKeyByName(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name && item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
			return item.ValueFrom.SecretKeyRef.Name + "/" + item.ValueFrom.SecretKeyRef.Key
		}
	}
	return ""
}
