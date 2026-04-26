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
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/directupload"
	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

const defaultDirectUploadOrphanStaleAge = directupload.DefaultSessionTTL + DefaultActivationDelay

type directUploadInventory struct {
	StoredPrefixCount    int
	ProtectedPrefixCount int
	StalePrefixes        []PrefixInventoryEntry
}

type directUploadStoredEntry struct {
	Prefix          string
	ObjectCount     int
	SampleObjectKey string
	LastModifiedAt  time.Time
}

func discoverDirectUploadInventory(
	ctx context.Context,
	store prefixStore,
	rootDirectory string,
	now time.Time,
	policy cleanupPolicy,
) (directUploadInventory, error) {
	policy = normalizeCleanupPolicy(policy)

	storedPrefixes, err := collectStoredDirectUploadPrefixes(ctx, store, rootDirectory)
	if err != nil {
		return directUploadInventory{}, err
	}

	protectedPrefixes, err := collectProtectedDirectUploadPrefixes(ctx, store, rootDirectory)
	if err != nil {
		return directUploadInventory{}, err
	}

	return directUploadInventory{
		StoredPrefixCount:    len(storedPrefixes),
		ProtectedPrefixCount: len(protectedPrefixes),
		StalePrefixes:        staleDirectUploadPrefixes(storedPrefixes, protectedPrefixes, now.UTC(), policy),
	}, nil
}

func collectStoredDirectUploadPrefixes(
	ctx context.Context,
	store prefixStore,
	rootDirectory string,
) ([]directUploadStoredEntry, error) {
	entriesByPrefix := make(map[string]directUploadStoredEntry)
	if err := store.ForEachObjectInfo(ctx, directUploadInventoryBasePrefix(rootDirectory), func(info prefixObjectInfo) {
		prefix, ok := inferDirectUploadPrefix(rootDirectory, info.Key)
		if !ok {
			return
		}
		entry := entriesByPrefix[prefix]
		entry.Prefix = prefix
		entry.ObjectCount++
		if entry.SampleObjectKey == "" {
			entry.SampleObjectKey = strings.Trim(strings.TrimSpace(info.Key), "/")
		}
		if info.LastModified.After(entry.LastModifiedAt) {
			entry.LastModifiedAt = info.LastModified.UTC()
		}
		entriesByPrefix[prefix] = entry
	}); err != nil {
		return nil, fmt.Errorf("discover direct-upload object prefixes: %w", err)
	}

	result := make([]directUploadStoredEntry, 0, len(entriesByPrefix))
	for _, entry := range entriesByPrefix {
		if entry.LastModifiedAt.IsZero() {
			return nil, fmt.Errorf("direct-upload prefix %s is missing last-modified timestamp", entry.Prefix)
		}
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Prefix < result[j].Prefix
	})
	return result, nil
}

func collectProtectedDirectUploadPrefixes(ctx context.Context, store prefixStore, rootDirectory string) (map[string]struct{}, error) {
	metadataKeys := make([]string, 0, 128)
	if err := store.ForEachObject(ctx, blobInventoryBasePrefix(rootDirectory), func(key string) {
		if sealedblob.IsMetadataPath(key) {
			metadataKeys = append(metadataKeys, strings.Trim(strings.TrimSpace(key), "/"))
		}
	}); err != nil {
		return nil, fmt.Errorf("list sealed blob metadata objects: %w", err)
	}

	sort.Strings(metadataKeys)
	protected := make(map[string]struct{}, len(metadataKeys))
	for _, key := range metadataKeys {
		payload, err := store.GetObject(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("read sealed blob metadata %s: %w", key, err)
		}
		metadata, err := sealedblob.Unmarshal(payload)
		if err != nil {
			return nil, fmt.Errorf("decode sealed blob metadata %s: %w", key, err)
		}
		prefix, ok := inferDirectUploadPrefix(rootDirectory, metadata.PhysicalPath)
		if !ok {
			continue
		}
		protected[prefix] = struct{}{}
	}
	return protected, nil
}

func staleDirectUploadPrefixes(
	stored []directUploadStoredEntry,
	protected map[string]struct{},
	now time.Time,
	policy cleanupPolicy,
) []PrefixInventoryEntry {
	cutoff := now.Add(-policy.directUploadStaleAge)
	stale := make([]PrefixInventoryEntry, 0, len(stored))
	for _, entry := range stored {
		if _, found := protected[entry.Prefix]; found {
			continue
		}
		if _, targeted := policy.targetDirectUploadPrefixes[entry.Prefix]; !targeted && entry.LastModifiedAt.After(cutoff) {
			continue
		}
		stale = append(stale, PrefixInventoryEntry{
			Prefix:          entry.Prefix,
			ObjectCount:     entry.ObjectCount,
			SampleObjectKey: entry.SampleObjectKey,
		})
	}
	return stale
}

func normalizeCleanupPolicy(policy cleanupPolicy) cleanupPolicy {
	if policy.directUploadStaleAge <= 0 {
		policy.directUploadStaleAge = defaultDirectUploadOrphanStaleAge
	}
	if policy.targetDirectUploadPrefixes == nil {
		policy.targetDirectUploadPrefixes = make(map[string]struct{})
	} else {
		normalizedTargets := make(map[string]struct{}, len(policy.targetDirectUploadPrefixes))
		for prefix := range policy.targetDirectUploadPrefixes {
			if normalized := normalizeDirectUploadPrefixTarget(prefix); normalized != "" {
				normalizedTargets[normalized] = struct{}{}
			}
		}
		policy.targetDirectUploadPrefixes = normalizedTargets
	}
	if policy.targetDirectUploadMultipartUploads == nil {
		policy.targetDirectUploadMultipartUploads = make(map[directUploadMultipartTarget]struct{})
	} else {
		normalizedTargets := make(map[directUploadMultipartTarget]struct{}, len(policy.targetDirectUploadMultipartUploads))
		for target := range policy.targetDirectUploadMultipartUploads {
			normalizedTargets[normalizeDirectUploadMultipartTarget(target)] = struct{}{}
		}
		policy.targetDirectUploadMultipartUploads = normalizedTargets
	}
	return policy
}

func normalizeDirectUploadPrefixTarget(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

func directUploadInventoryBasePrefix(rootDirectory string) string {
	return withOptionalRoot(rootDirectory, path.Join("_ai_models", "direct-upload", "objects"))
}

func blobInventoryBasePrefix(rootDirectory string) string {
	return withOptionalRoot(rootDirectory, "docker/registry/v2/blobs")
}

func inferDirectUploadPrefix(rootDirectory string, rawPath string) (string, bool) {
	cleanPath := strings.Trim(strings.TrimSpace(rawPath), "/")
	cleanRoot := strings.Trim(strings.TrimSpace(rootDirectory), "/")
	if cleanRoot != "" && strings.HasPrefix(cleanPath, cleanRoot+"/") {
		cleanPath = strings.TrimPrefix(cleanPath, cleanRoot+"/")
	}

	segments := splitKey(cleanPath)
	if len(segments) < 5 {
		return "", false
	}
	if segments[0] != "_ai_models" || segments[1] != "direct-upload" || segments[2] != "objects" {
		return "", false
	}
	if strings.TrimSpace(segments[3]) == "" {
		return "", false
	}
	return withOptionalRoot(rootDirectory, path.Join(segments[:4]...)), true
}
