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

package nodecacheintent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const DataKey = "intents.json"

type ArtifactIntent struct {
	ArtifactURI string `json:"artifactURI"`
	Digest      string `json:"digest"`
	Family      string `json:"family,omitempty"`
}

func NormalizeIntents(intents []ArtifactIntent) ([]ArtifactIntent, error) {
	if len(intents) == 0 {
		return nil, nil
	}

	normalized := make([]ArtifactIntent, 0, len(intents))
	seen := map[string]ArtifactIntent{}

	for _, intent := range intents {
		intent = ArtifactIntent{
			ArtifactURI: strings.TrimSpace(intent.ArtifactURI),
			Digest:      strings.TrimSpace(intent.Digest),
			Family:      strings.TrimSpace(intent.Family),
		}
		switch {
		case intent.ArtifactURI == "":
			return nil, errors.New("node cache intent artifact URI must not be empty")
		case intent.Digest == "":
			return nil, errors.New("node cache intent digest must not be empty")
		}

		existing, found := seen[intent.Digest]
		if !found {
			seen[intent.Digest] = intent
			normalized = append(normalized, intent)
			continue
		}
		if existing.ArtifactURI != intent.ArtifactURI {
			return nil, fmt.Errorf("node cache intent digest %q maps to multiple artifact URIs", intent.Digest)
		}
		if existing.Family != "" && intent.Family != "" && existing.Family != intent.Family {
			return nil, fmt.Errorf("node cache intent digest %q maps to multiple artifact families", intent.Digest)
		}
		if existing.Family == "" && intent.Family != "" {
			seen[intent.Digest] = intent
			for index := range normalized {
				if normalized[index].Digest == intent.Digest {
					normalized[index] = intent
					break
				}
			}
		}
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Digest < normalized[j].Digest
	})
	return normalized, nil
}

func ProtectedDigests(intents []ArtifactIntent) []string {
	normalized, err := NormalizeIntents(intents)
	if err != nil {
		return nil
	}
	protected := make([]string, 0, len(normalized))
	for _, intent := range normalized {
		protected = append(protected, intent.Digest)
	}
	return protected
}

func EncodeIntents(intents []ArtifactIntent) ([]byte, error) {
	normalized, err := NormalizeIntents(intents)
	if err != nil {
		return nil, err
	}
	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func DecodeIntents(payload []byte) ([]ArtifactIntent, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var intents []ArtifactIntent
	if err := json.Unmarshal(payload, &intents); err != nil {
		return nil, err
	}
	return NormalizeIntents(intents)
}
