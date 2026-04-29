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

package nodecache

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type DesiredArtifact struct {
	ArtifactURI string
	Digest      string
	Family      string
	SizeBytes   int64
}

func NormalizeDesiredArtifacts(artifacts []DesiredArtifact) ([]DesiredArtifact, error) {
	if len(artifacts) == 0 {
		return nil, nil
	}

	normalized := make([]DesiredArtifact, 0, len(artifacts))
	seen := map[string]DesiredArtifact{}

	for _, artifact := range artifacts {
		artifact = DesiredArtifact{
			ArtifactURI: strings.TrimSpace(artifact.ArtifactURI),
			Digest:      strings.TrimSpace(artifact.Digest),
			Family:      strings.TrimSpace(artifact.Family),
			SizeBytes:   artifact.SizeBytes,
		}
		switch {
		case artifact.ArtifactURI == "":
			return nil, errors.New("node cache desired artifact URI must not be empty")
		case artifact.Digest == "":
			return nil, errors.New("node cache desired digest must not be empty")
		}

		existing, found := seen[artifact.Digest]
		if !found {
			seen[artifact.Digest] = artifact
			normalized = append(normalized, artifact)
			continue
		}
		merged, err := mergeDesiredArtifact(existing, artifact)
		if err != nil {
			return nil, err
		}
		if merged != existing {
			seen[artifact.Digest] = merged
			replaceDesiredArtifact(normalized, merged)
		}
	}

	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].Digest < normalized[j].Digest
	})
	return normalized, nil
}

func mergeDesiredArtifact(existing, artifact DesiredArtifact) (DesiredArtifact, error) {
	if existing.ArtifactURI != artifact.ArtifactURI {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact URIs", artifact.Digest)
	}
	if existing.Family != "" && artifact.Family != "" && existing.Family != artifact.Family {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact families", artifact.Digest)
	}
	if existing.SizeBytes > 0 && artifact.SizeBytes > 0 && existing.SizeBytes != artifact.SizeBytes {
		return DesiredArtifact{}, fmt.Errorf("node cache desired digest %q maps to multiple artifact sizes", artifact.Digest)
	}

	merged := existing
	if merged.Family == "" && artifact.Family != "" {
		merged.Family = artifact.Family
	}
	if merged.SizeBytes <= 0 && artifact.SizeBytes > 0 {
		merged.SizeBytes = artifact.SizeBytes
	}
	return merged, nil
}

func replaceDesiredArtifact(artifacts []DesiredArtifact, replacement DesiredArtifact) {
	for index := range artifacts {
		if artifacts[index].Digest == replacement.Digest {
			artifacts[index] = replacement
			return
		}
	}
}

func ProtectedDigests(artifacts []DesiredArtifact) []string {
	normalized, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil {
		return nil
	}
	protected := make([]string, 0, len(normalized))
	for _, artifact := range normalized {
		protected = append(protected, artifact.Digest)
	}
	return protected
}
