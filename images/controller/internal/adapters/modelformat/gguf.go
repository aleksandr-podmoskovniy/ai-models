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

package modelformat

import (
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func ggufRules() formatRules {
	return formatRules{
		format:              modelsv1alpha1.ModelInputFormatGGUF,
		classify:            classifyGGUFFile,
		requiredAssetErrMsg: "requires at least one .gguf file",
	}
}

func classifyGGUFFile(relative string) (fileAction, bool, bool) {
	base := filepath.Base(relative)
	lowerBase := strings.ToLower(base)

	if strings.HasPrefix(base, ".") {
		if base == ".gitattributes" {
			return fileActionDrop, false, false
		}
		return fileActionReject, false, false
	}
	if isBenignExtraFile(lowerBase) {
		return fileActionDrop, false, false
	}
	if hasForbiddenExtension(lowerBase) {
		return fileActionReject, false, false
	}
	if strings.HasSuffix(lowerBase, ".gguf") {
		return fileActionKeep, false, true
	}
	return fileActionReject, false, false
}
