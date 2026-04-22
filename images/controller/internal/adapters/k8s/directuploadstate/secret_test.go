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

package directuploadstate

import (
	"encoding/json"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func TestSecretRoundTrip(t *testing.T) {
	t.Parallel()

	secret, err := NewSecret(SecretSpec{
		Name:            "state-a",
		Namespace:       "d8-ai-models",
		OwnerGeneration: 7,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	state := modelpackports.DirectUploadState{
		PlannedLayerCount: 2,
		PlannedSizeBytes:  384,
		Phase:             modelpackports.DirectUploadStatePhaseRunning,
		Stage:             modelpackports.DirectUploadStateStageUploading,
		CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
			{
				Key:         "model|application/test",
				Digest:      "sha256:111",
				DiffID:      "sha256:111",
				SizeBytes:   128,
				MediaType:   "application/test",
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatRaw,
				Compression: modelpackports.LayerCompressionNone,
			},
		},
		CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
			Key:               "config|application/test",
			SessionToken:      "session-1",
			PartSizeBytes:     64,
			TotalSizeBytes:    256,
			UploadedSizeBytes: 128,
			DigestState:       []byte("hash-state"),
		},
	}

	payload, err := marshalState(state)
	if err != nil {
		t.Fatalf("marshalState() error = %v", err)
	}
	secret.Data[stateKey] = payload

	got, err := StateFromSecret(secret)
	if err != nil {
		t.Fatalf("StateFromSecret() error = %v", err)
	}
	if got.Phase != modelpackports.DirectUploadStatePhaseRunning {
		t.Fatalf("unexpected phase %q", got.Phase)
	}
	if got.Stage != modelpackports.DirectUploadStateStageUploading {
		t.Fatalf("unexpected stage %q", got.Stage)
	}
	if got.PlannedLayerCount != 2 || got.PlannedSizeBytes != 384 {
		t.Fatalf("unexpected planned progress state %#v", got)
	}
	if len(got.CompletedLayers) != 1 {
		t.Fatalf("unexpected completed layer count %d", len(got.CompletedLayers))
	}
	if got.CurrentLayer == nil || got.CurrentLayer.SessionToken != "session-1" {
		t.Fatalf("unexpected current layer %#v", got.CurrentLayer)
	}

	generation, err := OwnerGenerationFromSecret(secret)
	if err != nil {
		t.Fatalf("OwnerGenerationFromSecret() error = %v", err)
	}
	if generation != 7 {
		t.Fatalf("unexpected owner generation %d", generation)
	}
}

func TestStateFromSecretRejectsBrokenCurrentLayer(t *testing.T) {
	t.Parallel()

	secret, err := NewSecret(SecretSpec{
		Name:            "state-a",
		Namespace:       "d8-ai-models",
		OwnerGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	payload, err := json.Marshal(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseRunning,
		CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
			Key:               "model|application/test",
			PartSizeBytes:     64,
			TotalSizeBytes:    256,
			UploadedSizeBytes: 128,
		},
	})
	if err != nil {
		t.Fatalf("jsonMarshal() error = %v", err)
	}
	secret.Data[stateKey] = payload

	if _, err := StateFromSecret(secret); err == nil {
		t.Fatal("expected StateFromSecret() to reject running state without session token")
	}
}

func TestStateFromSecretInfersRunningStageForLegacyPayload(t *testing.T) {
	t.Parallel()

	secret, err := NewSecret(SecretSpec{
		Name:            "state-a",
		Namespace:       "d8-ai-models",
		OwnerGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	payload, err := json.Marshal(modelpackports.DirectUploadState{
		Phase: modelpackports.DirectUploadStatePhaseRunning,
		CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
			Key:               "model|application/test",
			SessionToken:      "session-1",
			PartSizeBytes:     64,
			TotalSizeBytes:    256,
			UploadedSizeBytes: 128,
			DigestState:       []byte("hash-state"),
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	secret.Data[stateKey] = payload

	got, err := StateFromSecret(secret)
	if err != nil {
		t.Fatalf("StateFromSecret() error = %v", err)
	}
	if got.Stage != modelpackports.DirectUploadStateStageUploading {
		t.Fatalf("unexpected inferred stage %q", got.Stage)
	}
}

func TestStateFromSecretRejectsUploadedBytesAbovePlannedTotal(t *testing.T) {
	t.Parallel()

	secret, err := NewSecret(SecretSpec{
		Name:            "state-a",
		Namespace:       "d8-ai-models",
		OwnerGeneration: 1,
	})
	if err != nil {
		t.Fatalf("NewSecret() error = %v", err)
	}

	payload, err := json.Marshal(modelpackports.DirectUploadState{
		PlannedLayerCount: 2,
		PlannedSizeBytes:  100,
		Phase:             modelpackports.DirectUploadStatePhaseRunning,
		CompletedLayers: []modelpackports.DirectUploadLayerDescriptor{
			{
				Key:         "model|application/test",
				Digest:      "sha256:111",
				DiffID:      "sha256:111",
				SizeBytes:   80,
				MediaType:   "application/test",
				TargetPath:  "model",
				Base:        modelpackports.LayerBaseModel,
				Format:      modelpackports.LayerFormatRaw,
				Compression: modelpackports.LayerCompressionNone,
			},
		},
		CurrentLayer: &modelpackports.DirectUploadCurrentLayer{
			Key:               "config|application/test",
			SessionToken:      "session-1",
			PartSizeBytes:     64,
			TotalSizeBytes:    64,
			UploadedSizeBytes: 32,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	secret.Data[stateKey] = payload

	if _, err := StateFromSecret(secret); err == nil {
		t.Fatal("expected StateFromSecret() to reject uploaded bytes above planned total")
	}
}
