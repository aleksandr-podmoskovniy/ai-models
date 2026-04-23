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

package garbagecollection

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

type PrefixInventoryEntry struct {
	Prefix          string
	ObjectCount     int
	SampleObjectKey string
}

type Report struct {
	LiveRepositoryPrefixCount         int
	LiveRawPrefixCount                int
	StoredRepositoryPrefixCount       int
	StoredRawPrefixCount              int
	StoredDirectUploadPrefixCount     int
	ReferencedDirectUploadPrefixCount int
	StaleRepositories                 []PrefixInventoryEntry
	StaleRawPrefixes                  []PrefixInventoryEntry
	StaleDirectUploadPrefixes         []PrefixInventoryEntry
}

type livePrefixSet struct {
	repositoryPrefixes map[string]struct{}
	rawPrefixes        map[string]struct{}
}

func newLivePrefixSet() livePrefixSet {
	return livePrefixSet{
		repositoryPrefixes: make(map[string]struct{}),
		rawPrefixes:        make(map[string]struct{}),
	}
}

func (s *livePrefixSet) addRepository(prefix string) {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return
	}
	s.repositoryPrefixes[cleanPrefix] = struct{}{}
}

func (s *livePrefixSet) addRaw(prefix string) {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return
	}
	s.rawPrefixes[cleanPrefix] = struct{}{}
}

func buildReport(
	live livePrefixSet,
	storedRepositories []PrefixInventoryEntry,
	storedRawPrefixes []PrefixInventoryEntry,
) Report {
	return Report{
		LiveRepositoryPrefixCount:   len(live.repositoryPrefixes),
		LiveRawPrefixCount:          len(live.rawPrefixes),
		StoredRepositoryPrefixCount: len(storedRepositories),
		StoredRawPrefixCount:        len(storedRawPrefixes),
		StaleRepositories:           staleEntries(live.repositoryPrefixes, storedRepositories),
		StaleRawPrefixes:            staleEntries(live.rawPrefixes, storedRawPrefixes),
	}
}

func staleEntries(live map[string]struct{}, stored []PrefixInventoryEntry) []PrefixInventoryEntry {
	stale := make([]PrefixInventoryEntry, 0, len(stored))
	for _, entry := range stored {
		cleanPrefix := strings.Trim(strings.TrimSpace(entry.Prefix), "/")
		if cleanPrefix == "" {
			continue
		}
		if _, found := live[cleanPrefix]; found {
			continue
		}
		stale = append(stale, PrefixInventoryEntry{
			Prefix:          cleanPrefix,
			ObjectCount:     entry.ObjectCount,
			SampleObjectKey: strings.Trim(strings.TrimSpace(entry.SampleObjectKey), "/"),
		})
	}
	sort.Slice(stale, func(i, j int) bool {
		return stale[i].Prefix < stale[j].Prefix
	})
	return stale
}

func (r Report) HasStalePrefixes() bool {
	return len(r.StaleRepositories) > 0 || len(r.StaleRawPrefixes) > 0 || len(r.StaleDirectUploadPrefixes) > 0
}

func (r Report) Format() string {
	lines := []string{
		fmt.Sprintf("Live repository prefixes: %d", r.LiveRepositoryPrefixCount),
		fmt.Sprintf("Live raw source mirror prefixes: %d", r.LiveRawPrefixCount),
		fmt.Sprintf("Stored repository prefixes: %d", r.StoredRepositoryPrefixCount),
		fmt.Sprintf("Stored raw source mirror prefixes: %d", r.StoredRawPrefixCount),
		fmt.Sprintf("Stored direct-upload prefixes: %d", r.StoredDirectUploadPrefixCount),
		fmt.Sprintf("Referenced direct-upload prefixes: %d", r.ReferencedDirectUploadPrefixCount),
		fmt.Sprintf("Stale repository prefixes: %d", len(r.StaleRepositories)),
		fmt.Sprintf("Stale raw source mirror prefixes: %d", len(r.StaleRawPrefixes)),
		fmt.Sprintf("Stale orphan direct-upload prefixes: %d", len(r.StaleDirectUploadPrefixes)),
	}
	if !r.HasStalePrefixes() {
		lines = append(lines, "No stale prefixes eligible for cleanup.")
		return strings.Join(lines, "\n") + "\n"
	}

	if len(r.StaleRepositories) > 0 {
		lines = append(lines, "", "Stale repository prefixes:")
		for _, entry := range r.StaleRepositories {
			lines = append(lines, formatReportEntry(entry))
		}
	}
	if len(r.StaleRawPrefixes) > 0 {
		lines = append(lines, "", "Stale raw source mirror prefixes:")
		for _, entry := range r.StaleRawPrefixes {
			lines = append(lines, formatReportEntry(entry))
		}
	}
	if len(r.StaleDirectUploadPrefixes) > 0 {
		lines = append(lines, "", "Stale orphan direct-upload prefixes:")
		for _, entry := range r.StaleDirectUploadPrefixes {
			lines = append(lines, formatReportEntry(entry))
		}
	}

	return strings.Join(lines, "\n") + "\n"
}

func formatReportEntry(entry PrefixInventoryEntry) string {
	parts := []string{
		"- " + strings.Trim(strings.TrimSpace(entry.Prefix), "/"),
		fmt.Sprintf("objects=%d", entry.ObjectCount),
	}
	if sample := strings.Trim(strings.TrimSpace(entry.SampleObjectKey), "/"); sample != "" {
		parts = append(parts, "sample="+sample)
	}
	return strings.Join(parts, " ")
}

func repositoryMetadataPrefixFromReference(reference string) string {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repository, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" {
		return ""
	}
	repository = strings.TrimSpace(repository)
	if repository == "" {
		return ""
	}
	repositoryPart := repository[strings.LastIndex(repository, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		repository = repository[:strings.LastIndex(repository, ":")]
	}
	repository = strings.Trim(repository, "/")
	if repository == "" {
		return ""
	}
	return path.Join("dmcr", "docker", "registry", "v2", "repositories", repository)
}
