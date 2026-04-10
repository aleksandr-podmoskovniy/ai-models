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
	"encoding/base64"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestBuildAcceptsHuggingFacePublicationRequest(t *testing.T) {
	t.Parallel()

	request := testOperationContext()
	request.Request.Spec.Source.URL = "https://huggingface.co/deepseek-ai/DeepSeek-R1?revision=main"

	options := testOptions()
	options.OCIRegistryCASecretName = "ai-models-dmcr-ca"

	pod, err := Build(request, options, "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got, want := pod.Spec.Containers[0].Args[0], "publish-worker"; got != want {
		t.Fatalf("unexpected subcommand %q", got)
	}
	if got, want := pod.Spec.ServiceAccountName, "ai-models-controller"; got != want {
		t.Fatalf("unexpected service account %q", got)
	}
}

func TestBuildAcceptsHTTPPublicationRequest(t *testing.T) {
	t.Parallel()

	request := testOperationContext()
	request.Request.Owner.UID = "1111-3333"
	request.Request.Owner.Name = "deepseek-r1-http"
	request.Request.Identity.Name = "deepseek-r1-http"
	request.Request.Spec.Source.URL = "https://downloads.example/models/deepseek-r1.tar.gz"
	request.Request.Spec.Source.CABundle = []byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")

	pod, err := Build(request, testOptions(), "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	args := pod.Spec.Containers[0].Args
	assertContains(t, args, "--http-url")
	assertContains(t, args, "https://downloads.example/models/deepseek-r1.tar.gz")
	assertContains(t, args, "--http-ca-bundle-b64")
	assertContains(t, args, base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n")))
}

func TestBuildIncludesHuggingFaceAuthTokenEnvFromProjectedSecret(t *testing.T) {
	t.Parallel()

	request := testOperationContext()
	request.Request.Owner.UID = "1111-3334"
	request.Request.Owner.Name = "deepseek-r1-hf-auth"
	request.Request.Identity.Name = "deepseek-r1-hf-auth"
	request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}

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

	request := testOperationContext()
	request.Request.Owner.UID = "1111-3334"
	request.Request.Owner.Name = "deepseek-r1-hf-auth"
	request.Request.Identity.Name = "deepseek-r1-hf-auth"
	request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "hf-auth"}

	if _, err := Build(request, testOptions(), ""); err == nil {
		t.Fatal("expected missing projected auth secret name to fail")
	}
}

func TestBuildRejectsHTTPWithoutTask(t *testing.T) {
	t.Parallel()

	request := testOperationContext()
	request.Request.Owner.UID = "1111-3335"
	request.Request.Owner.Name = "deepseek-r1-http-no-task"
	request.Request.Identity.Name = "deepseek-r1-http-no-task"
	request.Request.Spec.Source.URL = "https://downloads.example/models/deepseek-r1.tar.gz"
	request.Request.Spec.RuntimeHints = nil

	if _, err := Build(request, testOptions(), ""); err == nil {
		t.Fatal("expected HTTP source without task to be rejected")
	}
}

func TestBuildIncludesHTTPAuthSecretVolumeAndArgs(t *testing.T) {
	t.Parallel()

	request := testOperationContext()
	request.Request.Owner.UID = "1111-3336"
	request.Request.Owner.Name = "deepseek-r1-http-auth"
	request.Request.Identity.Name = "deepseek-r1-http-auth"
	request.Request.Spec.Source.URL = "https://downloads.example/models/deepseek-r1.tar.gz"
	request.Request.Spec.Source.AuthSecretRef = &modelsv1alpha1.SecretReference{Name: "http-auth"}

	pod, err := Build(request, testOptions(), "ai-model-publish-auth-1111-3336")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	assertContains(t, pod.Spec.Containers[0].Args, "--http-auth-dir")
	assertContains(t, pod.Spec.Containers[0].Args, httpAuthMountPath)

	for _, mount := range pod.Spec.Containers[0].VolumeMounts {
		if mount.Name == httpAuthVolumeName && mount.MountPath == httpAuthMountPath && mount.ReadOnly {
			goto volumeCheck
		}
	}
	t.Fatal("expected HTTP auth secret volume mount")

volumeCheck:
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == httpAuthVolumeName && volume.Secret != nil && volume.Secret.SecretName == "ai-model-publish-auth-1111-3336" {
			return
		}
	}
	t.Fatal("expected HTTP auth secret volume")
}

func TestBuildTruncatesOwnerLabelsToKubernetesLimit(t *testing.T) {
	t.Parallel()

	longName := strings.Repeat("a", 80)
	request := testOperationContext()
	request.Request.Owner.UID = "1111-4444"
	request.Request.Owner.Name = longName
	request.Request.Identity.Name = longName

	pod, err := Build(request, testOptions(), "")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if got := len(pod.Labels["ai-models.deckhouse.io/owner-name"]); got > 63 {
		t.Fatalf("owner-name label length = %d, want <= 63", got)
	}
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
