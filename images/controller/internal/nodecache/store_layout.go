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
	"path/filepath"
	"strings"
)

const (
	StoreDirName     = "store"
	CurrentLinkName  = "current"
	WorkloadLinkName = "model"
	MarkerFileName   = ".ai-models-materialized.json"
	LastUsedFileName = ".ai-models-last-used"
)

func StoreRoot(cacheRoot string) string {
	return filepath.Join(filepath.Clean(strings.TrimSpace(cacheRoot)), StoreDirName)
}

func StorePath(cacheRoot, digest string) string {
	return filepath.Join(StoreRoot(cacheRoot), strings.TrimSpace(digest))
}

func DigestFromArtifactURI(artifactURI string) string {
	artifactURI = strings.TrimSpace(artifactURI)
	if artifactURI == "" {
		return ""
	}
	before, after, ok := strings.Cut(artifactURI, "@")
	if !ok || strings.TrimSpace(before) == "" {
		return ""
	}
	return strings.TrimSpace(after)
}
