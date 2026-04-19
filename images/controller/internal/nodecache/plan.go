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
	"sort"
	"strings"
	"time"
)

type PlanInput struct {
	Now               time.Time
	MaxTotalSizeBytes int64
	MaxUnusedAge      time.Duration
	ProtectedDigests  []string
}

type CandidateReason string

const (
	CandidateReasonMalformed    CandidateReason = "malformed"
	CandidateReasonUnusedAge    CandidateReason = "unused-age"
	CandidateReasonSizePressure CandidateReason = "size-pressure"
)

type Candidate struct {
	Digest         string
	DestinationDir string
	ReclaimBytes   int64
	Reason         CandidateReason
	Failure        string
}

type EvictionPlan struct {
	TotalSizeBytes    int64
	ResidualSizeBytes int64
	ReclaimBytes      int64
	Candidates        []Candidate
}

func PlanEviction(snapshot Snapshot, input PlanInput) EvictionPlan {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	plan := EvictionPlan{
		TotalSizeBytes:    snapshot.TotalSizeBytes,
		ResidualSizeBytes: snapshot.TotalSizeBytes,
	}
	selected := map[string]struct{}{}
	protected := protectedDigestSet(input.ProtectedDigests)

	appendCandidate := func(entry Entry, reason CandidateReason) {
		key := entry.DestinationDir
		if _, exists := selected[key]; exists {
			return
		}
		selected[key] = struct{}{}
		plan.Candidates = append(plan.Candidates, Candidate{
			Digest:         entry.Digest,
			DestinationDir: entry.DestinationDir,
			ReclaimBytes:   entry.SizeBytes,
			Reason:         reason,
			Failure:        entry.Failure,
		})
		plan.ReclaimBytes += entry.SizeBytes
		plan.ResidualSizeBytes -= entry.SizeBytes
	}

	for _, entry := range malformedCandidates(snapshot.Entries, protected) {
		appendCandidate(entry, CandidateReasonMalformed)
	}
	for _, entry := range unusedAgeCandidates(snapshot.Entries, input, protected) {
		appendCandidate(entry, CandidateReasonUnusedAge)
	}
	if input.MaxTotalSizeBytes > 0 && plan.ResidualSizeBytes > input.MaxTotalSizeBytes {
		for _, entry := range sizePressureCandidates(snapshot.Entries, protected) {
			appendCandidate(entry, CandidateReasonSizePressure)
			if plan.ResidualSizeBytes <= input.MaxTotalSizeBytes {
				break
			}
		}
	}
	return plan
}

func malformedCandidates(entries []Entry, protected map[string]struct{}) []Entry {
	candidates := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if isProtectedDigest(entry.Digest, protected) {
			continue
		}
		if entry.Current || entry.Ready {
			continue
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].SizeBytes == candidates[j].SizeBytes {
			return candidates[i].DestinationDir < candidates[j].DestinationDir
		}
		return candidates[i].SizeBytes > candidates[j].SizeBytes
	})
	return candidates
}

func unusedAgeCandidates(entries []Entry, input PlanInput, protected map[string]struct{}) []Entry {
	if input.MaxUnusedAge <= 0 {
		return nil
	}
	candidates := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if isProtectedDigest(entry.Digest, protected) {
			continue
		}
		if !entry.Ready || entry.Current {
			continue
		}
		lastUsed := entryLastUsed(entry)
		if lastUsed.IsZero() || input.Now.Sub(lastUsed) < input.MaxUnusedAge {
			continue
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := entryLastUsed(candidates[i])
		right := entryLastUsed(candidates[j])
		if left.Equal(right) {
			return candidates[i].DestinationDir < candidates[j].DestinationDir
		}
		return left.Before(right)
	})
	return candidates
}

func sizePressureCandidates(entries []Entry, protected map[string]struct{}) []Entry {
	candidates := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if isProtectedDigest(entry.Digest, protected) {
			continue
		}
		if !entry.Ready || entry.Current {
			continue
		}
		candidates = append(candidates, entry)
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := entryLastUsed(candidates[i])
		right := entryLastUsed(candidates[j])
		switch {
		case left.Equal(right):
			if candidates[i].SizeBytes == candidates[j].SizeBytes {
				return candidates[i].DestinationDir < candidates[j].DestinationDir
			}
			return candidates[i].SizeBytes > candidates[j].SizeBytes
		default:
			return left.Before(right)
		}
	})
	return candidates
}

func entryLastUsed(entry Entry) time.Time {
	if !entry.LastUsedAt.IsZero() {
		return entry.LastUsedAt.UTC()
	}
	if !entry.ReadyAt.IsZero() {
		return entry.ReadyAt.UTC()
	}
	return time.Time{}
}

func protectedDigestSet(digests []string) map[string]struct{} {
	if len(digests) == 0 {
		return nil
	}
	protected := make(map[string]struct{}, len(digests))
	for _, digest := range digests {
		digest = strings.TrimSpace(digest)
		if digest == "" {
			continue
		}
		protected[digest] = struct{}{}
	}
	return protected
}

func isProtectedDigest(digest string, protected map[string]struct{}) bool {
	if len(protected) == 0 {
		return false
	}
	_, found := protected[strings.TrimSpace(digest)]
	return found
}
