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
	"fmt"
	"strings"
	"time"
)

const RuntimeUsageSummaryVersion = "v1"

type RuntimeUsageArtifact struct {
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

type RuntimeUsageSummary struct {
	Version                  string                 `json:"version"`
	NodeName                 string                 `json:"nodeName"`
	LimitBytes               int64                  `json:"limitBytes,omitempty"`
	UsedBytes                int64                  `json:"usedBytes"`
	BudgetAvailableBytes     int64                  `json:"budgetAvailableBytes,omitempty"`
	FilesystemAvailableBytes int64                  `json:"filesystemAvailableBytes,omitempty"`
	AvailableBytes           int64                  `json:"availableBytes,omitempty"`
	EntryCount               int                    `json:"entryCount"`
	ReadyEntryCount          int                    `json:"readyEntryCount"`
	ReadyArtifacts           []RuntimeUsageArtifact `json:"readyArtifacts,omitempty"`
	UpdatedAt                time.Time              `json:"updatedAt"`
}

func NewRuntimeUsageSummary(nodeName string, limitBytes int64, result MaintenanceResult, now time.Time) RuntimeUsageSummary {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	evicted := evictedDestinationDirs(result.Plan)
	usedBytes := result.Plan.ResidualSizeBytes
	if usedBytes < 0 {
		usedBytes = 0
	}
	summary := RuntimeUsageSummary{
		Version:        RuntimeUsageSummaryVersion,
		NodeName:       strings.TrimSpace(nodeName),
		LimitBytes:     limitBytes,
		UsedBytes:      usedBytes,
		EntryCount:     len(result.Snapshot.Entries) - len(evicted),
		ReadyArtifacts: make([]RuntimeUsageArtifact, 0, len(result.Snapshot.Entries)),
		UpdatedAt:      now.UTC(),
	}
	if summary.EntryCount < 0 {
		summary.EntryCount = 0
	}
	if limitBytes > 0 && usedBytes < limitBytes {
		summary.BudgetAvailableBytes = limitBytes - usedBytes
		summary.AvailableBytes = summary.BudgetAvailableBytes
	}
	for _, entry := range result.Snapshot.Entries {
		if _, found := evicted[entry.DestinationDir]; found || !entry.Ready {
			continue
		}
		summary.ReadyEntryCount++
		summary.ReadyArtifacts = append(summary.ReadyArtifacts, RuntimeUsageArtifact{
			Digest:    strings.TrimSpace(entry.Digest),
			SizeBytes: entry.SizeBytes,
		})
	}
	return summary
}

func (s *RuntimeUsageSummary) ApplyFilesystemAvailableBytes(availableBytes int64) {
	if availableBytes <= 0 {
		return
	}
	s.FilesystemAvailableBytes = availableBytes
	if s.AvailableBytes == 0 || availableBytes < s.AvailableBytes {
		s.AvailableBytes = availableBytes
	}
}

func MissingSizeBytes(summary RuntimeUsageSummary, artifacts []DesiredArtifact) (int64, error) {
	normalized, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil {
		return 0, err
	}
	ready := readyArtifactDigestSet(summary.ReadyArtifacts)
	var missingBytes int64
	for _, artifact := range normalized {
		if artifact.SizeBytes <= 0 {
			return 0, fmt.Errorf("node cache capacity cannot be checked without artifact size for digest %q", artifact.Digest)
		}
		if _, found := ready[artifact.Digest]; found {
			continue
		}
		missingBytes += artifact.SizeBytes
	}
	return missingBytes, nil
}

func evictedDestinationDirs(plan EvictionPlan) map[string]struct{} {
	if len(plan.Candidates) == 0 {
		return nil
	}
	evicted := make(map[string]struct{}, len(plan.Candidates))
	for _, candidate := range plan.Candidates {
		evicted[candidate.DestinationDir] = struct{}{}
	}
	return evicted
}

func readyArtifactDigestSet(artifacts []RuntimeUsageArtifact) map[string]struct{} {
	if len(artifacts) == 0 {
		return nil
	}
	ready := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		digest := strings.TrimSpace(artifact.Digest)
		if digest == "" {
			continue
		}
		ready[digest] = struct{}{}
	}
	return ready
}
