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
	"path"
	"sort"
	"strings"
)

func DiscoverStoredPrefixes(
	ctx context.Context,
	store prefixStore,
	rootDirectory string,
) ([]PrefixInventoryEntry, []PrefixInventoryEntry, error) {
	repositories, err := collectStoredPrefixes(
		ctx,
		store,
		repositoryInventoryBasePrefix(rootDirectory),
		func(key string) (string, bool) {
			return inferRepositoryMetadataPrefix(rootDirectory, key)
		},
	)
	if err != nil {
		return nil, nil, err
	}

	rawPrefixes, err := collectStoredPrefixes(
		ctx,
		store,
		"raw",
		func(key string) (string, bool) {
			return inferSourceMirrorPrefix(key)
		},
	)
	if err != nil {
		return nil, nil, err
	}

	return repositories, rawPrefixes, nil
}

func collectStoredPrefixes(
	ctx context.Context,
	store prefixStore,
	basePrefix string,
	infer func(string) (string, bool),
) ([]PrefixInventoryEntry, error) {
	entriesByPrefix := make(map[string]PrefixInventoryEntry)
	if err := store.ForEachObject(ctx, basePrefix, func(key string) {
		prefix, ok := infer(key)
		if !ok {
			return
		}
		entry := entriesByPrefix[prefix]
		entry.Prefix = prefix
		entry.ObjectCount++
		if entry.SampleObjectKey == "" {
			entry.SampleObjectKey = strings.Trim(strings.TrimSpace(key), "/")
		}
		entriesByPrefix[prefix] = entry
	}); err != nil {
		return nil, err
	}

	result := make([]PrefixInventoryEntry, 0, len(entriesByPrefix))
	for _, entry := range entriesByPrefix {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Prefix < result[j].Prefix
	})
	return result, nil
}

func repositoryInventoryBasePrefix(rootDirectory string) string {
	return path.Join(withOptionalRoot(rootDirectory, "docker/registry/v2/repositories"), "ai-models", "catalog")
}

func inferRepositoryMetadataPrefix(rootDirectory string, objectKey string) (string, bool) {
	basePrefix := withOptionalRoot(rootDirectory, "docker/registry/v2/repositories")
	cleanKey := strings.Trim(strings.TrimSpace(objectKey), "/")
	cleanBase := strings.Trim(strings.TrimSpace(basePrefix), "/")
	if cleanKey == cleanBase || !strings.HasPrefix(cleanKey, cleanBase+"/") {
		return "", false
	}

	relative := strings.TrimPrefix(cleanKey, cleanBase+"/")
	segments := splitKey(relative)
	if len(segments) < 5 || segments[0] != "ai-models" || segments[1] != "catalog" {
		return "", false
	}

	switch segments[2] {
	case "cluster":
		if len(segments) < 5 {
			return "", false
		}
		return path.Join(append([]string{cleanBase}, segments[:5]...)...), true
	case "namespaced":
		if len(segments) < 6 {
			return "", false
		}
		return path.Join(append([]string{cleanBase}, segments[:6]...)...), true
	default:
		return "", false
	}
}

func inferSourceMirrorPrefix(objectKey string) (string, bool) {
	cleanKey := strings.Trim(strings.TrimSpace(objectKey), "/")
	if cleanKey == "" {
		return "", false
	}

	switch {
	case strings.HasSuffix(cleanKey, "/manifest.json"):
		prefix := strings.TrimSuffix(cleanKey, "/manifest.json")
		return cleanSourceMirrorPrefix(prefix)
	case strings.HasSuffix(cleanKey, "/state.json"):
		prefix := strings.TrimSuffix(cleanKey, "/state.json")
		return cleanSourceMirrorPrefix(prefix)
	default:
		index := strings.Index(cleanKey, "/files/")
		if index == -1 {
			return "", false
		}
		prefix := cleanKey[:index]
		return cleanSourceMirrorPrefix(prefix)
	}
}

func cleanSourceMirrorPrefix(prefix string) (string, bool) {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" || !strings.Contains(cleanPrefix, "/source-url/.mirror/") {
		return "", false
	}
	return cleanPrefix, true
}

func splitKey(raw string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(raw), "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}
	return result
}

func withOptionalRoot(rootDirectory string, objectPath string) string {
	cleanRoot := strings.Trim(strings.TrimSpace(rootDirectory), "/")
	cleanPath := strings.Trim(strings.TrimSpace(objectPath), "/")
	if cleanRoot == "" {
		return cleanPath
	}
	return path.Join(cleanRoot, cleanPath)
}
