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

package publicationartifact

import (
	"encoding/json"
	"errors"
	"strings"

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

// Result is the controller-owned publication runtime outcome encoded into the
// worker termination payload and decoded by the lifecycle controller.
type Result struct {
	Artifact      publication.PublishedArtifact
	Resolved      publication.ResolvedProfile
	Source        publication.SourceProvenance
	CleanupHandle cleanuphandle.Handle
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
		return Result{}, errors.New("publication artifact result payload must not be empty")
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
