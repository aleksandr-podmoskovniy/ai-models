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

package publishedsnapshot

import (
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

type Scope string

const (
	ScopeNamespaced Scope = "Namespaced"
	ScopeCluster    Scope = "Cluster"
)

type Identity struct {
	Scope     Scope
	Namespace string
	Name      string
}

type SourceProvenance struct {
	Type              modelsv1alpha1.ModelSourceType
	ExternalReference string
	ResolvedRevision  string
	RawURI            string
	RawObjectCount    int64
	RawSizeBytes      int64
}

type PublishedArtifact struct {
	Kind      modelsv1alpha1.ModelArtifactLocationKind
	URI       string
	Digest    string
	MediaType string
	SizeBytes int64
}

type ResolvedProfile struct {
	Task                          string
	TaskConfidence                ProfileConfidence
	SourceCapabilities            SourceCapabilities
	Family                        string
	FamilyConfidence              ProfileConfidence
	License                       string
	Architecture                  string
	ArchitectureConfidence        ProfileConfidence
	Format                        string
	ParameterCount                int64
	ParameterCountConfidence      ProfileConfidence
	Quantization                  string
	QuantizationConfidence        ProfileConfidence
	ContextWindowTokens           int64
	ContextWindowTokensConfidence ProfileConfidence
	SourceRepoID                  string
	SupportedEndpointTypes        []string
	SupportedFeatures             []string
	Footprint                     ProfileFootprint
}

type SourceCapabilities struct {
	Tasks    []string
	Features []string
}

type Result struct {
	State string
	Ready bool
}

type Snapshot struct {
	Identity Identity
	Source   SourceProvenance
	Artifact PublishedArtifact
	Resolved ResolvedProfile
	Result   Result
}

func (s Snapshot) Validate() error {
	if err := s.Identity.Validate(); err != nil {
		return err
	}

	if err := s.Source.Validate(); err != nil {
		return err
	}

	if err := s.Artifact.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(s.Result.State) == "" {
		return errors.New("publication result state must not be empty")
	}

	return nil
}

func (i Identity) Validate() error {
	if strings.TrimSpace(i.Name) == "" {
		return errors.New("publication identity name must not be empty")
	}

	switch i.Scope {
	case ScopeNamespaced:
		if strings.TrimSpace(i.Namespace) == "" {
			return errors.New("publication identity namespace must not be empty for namespaced scope")
		}
	case ScopeCluster:
		if strings.TrimSpace(i.Namespace) != "" {
			return errors.New("publication identity namespace must be empty for cluster scope")
		}
	default:
		return fmt.Errorf("unsupported publication scope %q", i.Scope)
	}

	return nil
}

func (s SourceProvenance) Validate() error {
	if strings.TrimSpace(string(s.Type)) == "" {
		return errors.New("source provenance type must not be empty")
	}
	if s.RawObjectCount < 0 {
		return errors.New("source provenance raw object count must not be negative")
	}
	if s.RawSizeBytes < 0 {
		return errors.New("source provenance raw size bytes must not be negative")
	}

	return nil
}

func (a PublishedArtifact) Validate() error {
	switch a.Kind {
	case modelsv1alpha1.ModelArtifactLocationKindOCI:
	default:
		return fmt.Errorf("unsupported published artifact kind %q", a.Kind)
	}

	if strings.TrimSpace(a.URI) == "" {
		return errors.New("published artifact URI must not be empty")
	}

	return nil
}

func (i Identity) Reference() string {
	if i.Scope == ScopeNamespaced {
		return i.Namespace + "/" + i.Name
	}

	return i.Name
}
