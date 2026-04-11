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
	"bytes"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

func TestValidateUploadProbe(t *testing.T) {
	t.Parallel()

	session := UploadSession{
		Owner: OwnerBinding{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1",
			Namespace: "team-a",
			UID:       "1111-2222",
		},
		Identity: publicationdata.Identity{
			Scope:     publicationdata.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
	}

	t.Run("gguf direct file resolves format", func(t *testing.T) {
		t.Parallel()
		session := session

		got, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.gguf",
			Chunk:    []byte("GGUFpayload"),
		})
		if err != nil {
			t.Fatalf("ValidateUploadProbe() error = %v", err)
		}
		if got.ResolvedInputFormat != modelsv1alpha1.ModelInputFormatGGUF {
			t.Fatalf("unexpected resolved format %q", got.ResolvedInputFormat)
		}
	})

	t.Run("archive is accepted for declared safetensors", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = modelsv1alpha1.ModelInputFormatSafetensors
		got, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.tar.gz",
			Chunk:    []byte{0x1f, 0x8b, 0x08, 0x00},
		})
		if err != nil {
			t.Fatalf("ValidateUploadProbe() error = %v", err)
		}
		if got.ResolvedInputFormat != modelsv1alpha1.ModelInputFormatSafetensors {
			t.Fatalf("unexpected resolved format %q", got.ResolvedInputFormat)
		}
	})

	t.Run("zip archives are accepted", func(t *testing.T) {
		t.Parallel()
		session := session

		got, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.zip",
			Chunk:    []byte("PK\x03\x04payload"),
		})
		if err != nil {
			t.Fatalf("ValidateUploadProbe() error = %v", err)
		}
		if got.FileName != "model.zip" {
			t.Fatalf("unexpected probe result %#v", got)
		}
	})

	t.Run("zip archives reject mismatched signatures", func(t *testing.T) {
		t.Parallel()
		session := session

		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.zip",
			Chunk:    []byte("plain"),
		})
		if err == nil {
			t.Fatal("expected invalid zip signature to fail")
		}
	})

	t.Run("direct safetensors is rejected", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = ""
		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.safetensors",
			Chunk:    []byte("header"),
		})
		if err == nil {
			t.Fatal("expected direct safetensors probe to fail")
		}
	})

	t.Run("negative expected size fails session validation", func(t *testing.T) {
		t.Parallel()
		session := session

		session.ExpectedSizeBytes = -1
		if err := ValidateUploadSession(session); err == nil {
			t.Fatal("expected negative expected size to fail")
		}
	})

	t.Run("invalid declared format fails session validation", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = modelsv1alpha1.ModelInputFormat("Broken")
		if err := ValidateUploadSession(session); err == nil {
			t.Fatal("expected invalid format to fail")
		}
	})

	t.Run("mismatched gguf extension is rejected", func(t *testing.T) {
		t.Parallel()
		session := session

		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.gguf",
			Chunk:    []byte("not-gguf"),
		})
		if err == nil {
			t.Fatal("expected mismatched gguf probe to fail")
		}
	})

	t.Run("unknown file without declared format is rejected", func(t *testing.T) {
		t.Parallel()
		session := session

		session.ExpectedSizeBytes = 0
		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.bin",
			Chunk:    []byte("random-bytes"),
		})
		if err == nil {
			t.Fatal("expected unknown direct file to fail")
		}
	})

	t.Run("declared gguf rejects non-gguf payload", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = modelsv1alpha1.ModelInputFormatGGUF
		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.bin",
			Chunk:    []byte("plain"),
		})
		if err == nil {
			t.Fatal("expected declared gguf mismatch to fail")
		}
	})

	t.Run("declared safetensors rejects non-archive payload", func(t *testing.T) {
		t.Parallel()
		session := session

		session.DeclaredInputFormat = modelsv1alpha1.ModelInputFormatSafetensors
		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.bin",
			Chunk:    []byte("plain"),
		})
		if err == nil {
			t.Fatal("expected declared safetensors mismatch to fail")
		}
	})

	t.Run("oversized probe is rejected", func(t *testing.T) {
		t.Parallel()
		session := session

		_, err := ValidateUploadProbe(session, UploadProbeInput{
			FileName: "model.gguf",
			Chunk:    bytes.Repeat([]byte("a"), MaxUploadProbeBytes+1),
		})
		if err == nil {
			t.Fatal("expected oversized probe to fail")
		}
	})
}
