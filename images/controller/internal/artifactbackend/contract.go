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

package artifactbackend

import (
	"encoding/json"
	"errors"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

// Request is the backend-facing publication handoff. It is intentionally
// independent from any concrete backend implementation so the current backend
// can later be replaced without changing lifecycle controllers or public API.
type Request struct {
	Identity publication.Identity
	Spec     modelsv1alpha1.ModelSpec
}

// Result is the backend-facing publication outcome consumed by the lifecycle
// controller after the backend implementation stores the artifact and inspects
// the model payload.
type Result struct {
	Artifact      publication.PublishedArtifact
	Resolved      publication.ResolvedProfile
	Source        publication.SourceProvenance
	CleanupHandle cleanuphandle.Handle
}

func (r Request) Validate() error {
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	if _, err := r.Spec.Source.DetectType(); err != nil {
		return err
	}

	return nil
}

func (r Result) Validate() error {
	if err := r.Source.Validate(); err != nil {
		return err
	}
	if err := r.Artifact.Validate(); err != nil {
		return err
	}

	return r.CleanupHandle.Validate()
}

func EncodeResult(result Result) (string, error) {
	if err := result.Validate(); err != nil {
		return "", err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func DecodeResult(raw string) (Result, error) {
	if strings.TrimSpace(raw) == "" {
		return Result{}, errors.New("artifact backend result payload must not be empty")
	}

	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return Result{}, err
	}
	if err := result.Validate(); err != nil {
		return Result{}, err
	}

	return result, nil
}
