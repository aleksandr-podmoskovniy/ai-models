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

package sourceworker

import (
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/workloadpod"
)

func TestBuildAcceptsHuggingFacePublicationRequest(t *testing.T) {
	t.Parallel()

	request := testOperationRequest()
	request.Spec.Source.URL = "https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=main"

	options := testOptions()
	options.OCIRegistryCASecretName = "ai-models-dmcr-ca"

	pod, err := Build(request, options, "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := pod.Spec.Containers[0].Args[0], "publish-worker"; got != want {
		t.Fatalf("unexpected subcommand %q", got)
	}
	assertContains(t, pod.Spec.Containers[0].Args, "--raw-stage-bucket")
	assertContains(t, pod.Spec.Containers[0].Args, "ai-models")
	assertContains(t, pod.Spec.Containers[0].Args, "--raw-stage-key-prefix")
	assertContains(t, pod.Spec.Containers[0].Args, "raw/1111-2222/source-url")
	if got, want := pod.Spec.ServiceAccountName, "ai-models-controller"; got != want {
		t.Fatalf("unexpected service account %q", got)
	}
	if len(pod.Spec.ImagePullSecrets) != 1 || pod.Spec.ImagePullSecrets[0].Name != "ai-models-module-registry" {
		t.Fatalf("unexpected imagePullSecrets %#v", pod.Spec.ImagePullSecrets)
	}
	if got, want := pod.Spec.Containers[0].Resources.Requests.Cpu().String(), "1"; got != want {
		t.Fatalf("unexpected cpu request %q", got)
	}
	foundTMPDIR := false
	foundLogFormat := false
	foundLogLevel := false
	for _, item := range pod.Spec.Containers[0].Env {
		if item.Name == "TMPDIR" {
			if got, want := item.Value, workloadpod.WorkVolumeMountPath; got != want {
				t.Fatalf("unexpected TMPDIR %q", got)
			}
			foundTMPDIR = true
		}
		if item.Name == "LOG_FORMAT" {
			if got, want := item.Value, "json"; got != want {
				t.Fatalf("unexpected LOG_FORMAT %q", got)
			}
			foundLogFormat = true
		}
		if item.Name == "LOG_LEVEL" {
			if got, want := item.Value, "debug"; got != want {
				t.Fatalf("unexpected LOG_LEVEL %q", got)
			}
			foundLogLevel = true
		}
	}
	if !foundTMPDIR {
		t.Fatal("expected TMPDIR env")
	}
	if !foundLogFormat {
		t.Fatal("expected LOG_FORMAT env")
	}
	if !foundLogLevel {
		t.Fatal("expected LOG_LEVEL env")
	}
}

func TestBuildIncludesHuggingFaceAuthTokenEnvFromProjectedSecret(t *testing.T) {
	t.Parallel()

	request := testOperationRequest()
	request.Owner.UID = "1111-3334"
	request.Owner.Name = "deepseek-r1-hf-auth"
	request.Identity.Name = "deepseek-r1-hf-auth"
	request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}

	pod, err := Build(request, testOptions(), "ai-model-publish-auth-1111-3334")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, item := range pod.Spec.Containers[0].Env {
		if item.Name != "HF_TOKEN" || item.ValueFrom == nil || item.ValueFrom.SecretKeyRef == nil {
			continue
		}
		if got, want := item.ValueFrom.SecretKeyRef.Name, "ai-model-publish-auth-1111-3334"; got != want {
			t.Fatalf("unexpected HF_TOKEN secret name %q", got)
		}
		if got, want := item.ValueFrom.SecretKeyRef.Key, "token"; got != want {
			t.Fatalf("unexpected HF_TOKEN secret key %q", got)
		}
		return
	}
	t.Fatal("expected HF_TOKEN env sourced from projected auth secret")
}

func TestBuildRejectsMissingProjectedAuthSecretName(t *testing.T) {
	t.Parallel()

	request := testOperationRequest()
	request.Owner.UID = "1111-3334"
	request.Owner.Name = "deepseek-r1-hf-auth"
	request.Identity.Name = "deepseek-r1-hf-auth"
	request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}

	if _, err := Build(request, testOptions(), ""); err == nil {
		t.Fatal("expected missing projected auth secret name to fail")
	}
}

func TestBuildTruncatesOwnerLabelsToKubernetesLimit(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("a", 80)
	request := testOperationRequest()
	request.Owner.UID = "1111-4444"
	request.Owner.Name = longName
	request.Identity.Name = longName

	pod, err := Build(request, testOptions(), "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got := len(pod.Labels["ai.deckhouse.io/owner-name"]); got > 63 {
		t.Fatalf("owner-name label length = %d, want <= 63", got)
	}
}

func TestBuildSupportsPersistentVolumeClaimWorkVolume(t *testing.T) {
	t.Parallel()

	options := testOptions()
	options.WorkVolume.Type = workloadpod.WorkVolumeTypePersistentVolumeClaim
	options.WorkVolume.PersistentVolumeClaimName = "ai-models-publication-work"

	pod, err := Build(testOperationRequest(), options, "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.Name != workloadpod.WorkVolumeName {
			continue
		}
		if volume.PersistentVolumeClaim == nil {
			t.Fatalf("expected pvc-backed work volume, got %#v", volume)
		}
		if got, want := volume.PersistentVolumeClaim.ClaimName, "ai-models-publication-work"; got != want {
			t.Fatalf("unexpected pvc claim name %q", got)
		}
		return
	}
	t.Fatal("expected work volume")
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}

	t.Fatalf("expected %q in %v", want, values)
}
