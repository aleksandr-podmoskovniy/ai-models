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

package resourcenames

import (
	"errors"
	"strings"
)

const nodeCacheIntentConfigMapPrefix = "ai-models-node-cache-intent-"

func NodeCacheIntentConfigMapName(nodeName string) (string, error) {
	nodeName = strings.TrimSpace(nodeName)
	if nodeName == "" {
		return "", errors.New("node name must not be empty")
	}
	replacer := strings.NewReplacer("_", "-", ".", "-", ":", "-")
	nodeName = strings.ToLower(replacer.Replace(nodeName))
	nodeName = strings.Trim(nodeName, "-")
	if nodeName == "" {
		return "", errors.New("node name normalized to an empty value")
	}
	if len(nodeName) > 40 {
		nodeName = strings.Trim(nodeName[:40], "-")
	}
	if nodeName == "" {
		return "", errors.New("node name normalized to an empty value")
	}
	return nodeCacheIntentConfigMapPrefix + nodeName, nil
}
