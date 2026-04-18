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

package ingestadmission

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestValidateOwnerBinding(t *testing.T) {
	t.Parallel()

	owner := OwnerBinding{
		Kind:      modelsv1alpha1.ModelKind,
		Name:      "deepseek-r1",
		Namespace: "team-a",
		UID:       "1111-2222",
	}
	identity := publicationdata.Identity{
		Scope:     publicationdata.ScopeNamespaced,
		Namespace: "team-a",
		Name:      "deepseek-r1",
	}
	if err := ValidateOwnerBinding(owner, identity); err != nil {
		t.Fatalf("ValidateOwnerBinding() error = %v", err)
	}

	owner.Namespace = "team-b"
	if err := ValidateOwnerBinding(owner, identity); err == nil {
		t.Fatal("expected mismatched namespace to fail")
	}

	clusterOwner := OwnerBinding{
		Kind: modelsv1alpha1.ClusterModelKind,
		Name: "deepseek-r1-global",
		UID:  "3333-4444",
	}
	clusterIdentity := publicationdata.Identity{
		Scope: publicationdata.ScopeCluster,
		Name:  "deepseek-r1-global",
	}
	if err := ValidateOwnerBinding(clusterOwner, clusterIdentity); err != nil {
		t.Fatalf("ValidateOwnerBinding(cluster) error = %v", err)
	}
	clusterOwner.Namespace = "must-be-empty"
	if err := ValidateOwnerBinding(clusterOwner, clusterIdentity); err == nil {
		t.Fatal("expected cluster owner namespace to fail")
	}

	invalidOwner := OwnerBinding{Name: "deepseek-r1", Namespace: "team-a", UID: "1111-2222"}
	if err := ValidateOwnerBinding(invalidOwner, identity); err == nil {
		t.Fatal("expected missing owner kind to fail")
	}
	invalidOwner = OwnerBinding{Kind: modelsv1alpha1.ModelKind, Namespace: "team-a", UID: "1111-2222"}
	if err := ValidateOwnerBinding(invalidOwner, identity); err == nil {
		t.Fatal("expected missing owner name to fail")
	}
	invalidOwner = OwnerBinding{Kind: modelsv1alpha1.ModelKind, Name: "deepseek-r1", Namespace: "team-a"}
	if err := ValidateOwnerBinding(invalidOwner, identity); err == nil {
		t.Fatal("expected missing owner UID to fail")
	}
	invalidOwner = OwnerBinding{Kind: modelsv1alpha1.ModelKind, Name: "deepseek-r1", UID: "1111-2222"}
	if err := ValidateOwnerBinding(invalidOwner, identity); err == nil {
		t.Fatal("expected missing namespaced owner namespace to fail")
	}
	invalidOwner = OwnerBinding{Kind: modelsv1alpha1.ModelKind, Name: "other", Namespace: "team-a", UID: "1111-2222"}
	if err := ValidateOwnerBinding(invalidOwner, identity); err == nil {
		t.Fatal("expected owner name mismatch to fail")
	}
}

func TestValidateDeclaredInputFormat(t *testing.T) {
	t.Parallel()

	if err := ValidateDeclaredInputFormat(modelsv1alpha1.ModelInputFormatGGUF); err != nil {
		t.Fatalf("ValidateDeclaredInputFormat() error = %v", err)
	}
	if err := ValidateDeclaredInputFormat(modelsv1alpha1.ModelInputFormat("Broken")); err == nil {
		t.Fatal("expected invalid format to fail")
	}
}

func TestValidateRemoteFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		fileName       string
		declaredFormat modelsv1alpha1.ModelInputFormat
		wantErr        bool
	}{
		{name: "archive accepted", fileName: "model.tar.gz"},
		{name: "zstd tar archive accepted", fileName: "model.tar.zst"},
		{name: "path is normalized", fileName: "/tmp/model.gguf"},
		{name: "gguf accepted", fileName: "model.gguf"},
		{name: "empty rejected", fileName: "", wantErr: true},
		{name: "gguf rejects safetensors declaration", fileName: "model.gguf", declaredFormat: modelsv1alpha1.ModelInputFormatSafetensors, wantErr: true},
		{name: "direct safetensors rejected", fileName: "model.safetensors", wantErr: true},
		{name: "hidden file rejected", fileName: ".env", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRemoteFileName(tc.fileName, tc.declaredFormat)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateRemoteFileName() error = %v", err)
			}
		})
	}
}
