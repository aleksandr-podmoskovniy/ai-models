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

package resourcenames

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestPrefixedName(t *testing.T) {
	t.Parallel()

	name, err := PrefixedName("ai-model-", types.UID("UID.With:Chars_And-Long-Long-Long-Long-Long"))
	if err != nil {
		t.Fatalf("PrefixedName() error = %v", err)
	}
	if !strings.HasPrefix(name, "ai-model-") {
		t.Fatalf("expected prefixed name, got %q", name)
	}
	if strings.ContainsAny(name, "_.:") {
		t.Fatalf("expected normalized name, got %q", name)
	}
}

func TestOwnerSuffixRejectsEmptyUID(t *testing.T) {
	t.Parallel()

	if _, err := OwnerSuffix(""); err == nil {
		t.Fatal("expected error for empty uid")
	}
}

func TestTruncateLabelValue(t *testing.T) {
	t.Parallel()

	value := TruncateLabelValue(strings.Repeat("a", 70))
	if len(value) > 63 {
		t.Fatalf("expected truncated value, got len=%d", len(value))
	}
}

func TestBoolString(t *testing.T) {
	t.Parallel()

	if BoolString(true) != "true" {
		t.Fatal("expected true string")
	}
	if BoolString(false) != "false" {
		t.Fatal("expected false string")
	}
}

func TestOwnerLabels(t *testing.T) {
	t.Parallel()

	labels := OwnerLabels("ai-models-publication", "Model", "deepseek-r1", types.UID("uid-1"), "team-a")
	if got, want := labels[AppNameLabelKey], "ai-models-publication"; got != want {
		t.Fatalf("unexpected app label %q", got)
	}
	if got, want := labels[OwnerKindLabelKey], "Model"; got != want {
		t.Fatalf("unexpected owner-kind label %q", got)
	}
	if got, want := labels[OwnerNameLabelKey], "deepseek-r1"; got != want {
		t.Fatalf("unexpected owner-name label %q", got)
	}
	if got, want := labels[OwnerUIDLabelKey], "uid-1"; got != want {
		t.Fatalf("unexpected owner-uid label %q", got)
	}
	if got, want := labels[OwnerNamespaceLabelKey], "team-a"; got != want {
		t.Fatalf("unexpected owner-namespace label %q", got)
	}
}

func TestOwnerUIDFromLabels(t *testing.T) {
	t.Parallel()

	if uid, ok := OwnerUIDFromLabels(nil); ok || uid != "" {
		t.Fatalf("expected no uid from nil labels, got %q ok=%v", uid, ok)
	}
	if uid, ok := OwnerUIDFromLabels(map[string]string{}); ok || uid != "" {
		t.Fatalf("expected no uid from empty labels, got %q ok=%v", uid, ok)
	}

	uid, ok := OwnerUIDFromLabels(map[string]string{OwnerUIDLabelKey: "  uid-1  "})
	if !ok || uid != "uid-1" {
		t.Fatalf("unexpected uid %q ok=%v", uid, ok)
	}
}

func TestOwnerAnnotations(t *testing.T) {
	t.Parallel()

	annotations := OwnerAnnotations("Model", "very-long-owner-name-that-must-not-be-truncated-in-annotations", "team-a")
	if got, want := annotations[OwnerKindAnnotationKey], "Model"; got != want {
		t.Fatalf("unexpected owner-kind annotation %q", got)
	}
	if got, want := annotations[OwnerNameAnnotationKey], "very-long-owner-name-that-must-not-be-truncated-in-annotations"; got != want {
		t.Fatalf("unexpected owner-name annotation %q", got)
	}
	if got, want := annotations[OwnerNamespaceAnnotationKey], "team-a"; got != want {
		t.Fatalf("unexpected owner-namespace annotation %q", got)
	}
}

func TestCanonicalResourceNames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		fn     func(types.UID) (string, error)
		prefix string
	}{
		{name: "source worker pod", fn: SourceWorkerPodName, prefix: "ai-model-publish-"},
		{name: "source worker auth secret", fn: SourceWorkerAuthSecretName, prefix: "ai-model-publish-auth-"},
		{name: "oci registry auth secret", fn: OCIRegistryAuthSecretName, prefix: "ai-model-oci-auth-"},
		{name: "oci registry ca secret", fn: OCIRegistryCASecretName, prefix: "ai-model-oci-ca-"},
		{name: "upload session pod", fn: UploadSessionPodName, prefix: "ai-model-upload-"},
		{name: "upload session service", fn: UploadSessionServiceName, prefix: "ai-model-upload-"},
		{name: "upload session secret", fn: UploadSessionSecretName, prefix: "ai-model-upload-auth-"},
		{name: "cleanup job", fn: CleanupJobName, prefix: "ai-model-cleanup-"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			name, err := tc.fn(types.UID("UID.One:Two_Three"))
			if err != nil {
				t.Fatalf("name function error = %v", err)
			}
			if !strings.HasPrefix(name, tc.prefix) {
				t.Fatalf("expected prefix %q, got %q", tc.prefix, name)
			}
			if strings.ContainsAny(name, "_.:") {
				t.Fatalf("expected normalized name, got %q", name)
			}
		})
	}

	if _, err := CleanupJobName(types.UID(":::")); err == nil {
		t.Fatal("expected normalized-empty UID to be rejected")
	}
}
