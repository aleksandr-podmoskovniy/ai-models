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

func safetensorsRules() formatRules {
	return formatRules{
		format:               modelsv1alpha1.ModelInputFormatSafetensors,
		classify:             classifySafetensorsFile,
		requiredConfigErrMsg: "requires root config.json",
		requiredAssetErrMsg:  "requires at least one .safetensors file",
	}
}

func classifySafetensorsFile(relative string) (fileAction, bool, bool) {
	base := filepath.Base(relative)
	lowerBase := strings.ToLower(base)
	lowerRelative := strings.ToLower(relative)

	if hasDroppedPathComponent(relative) {
		return fileActionDrop, false, false
	}
	if relative == "model_index.json" {
		return fileActionReject, false, false
	}
	if relative == "config.json" {
		return fileActionKeep, true, false
	}
	if isModelCompanionFile(lowerRelative, lowerBase) {
		return fileActionKeep, false, false
	}
	if hasDroppedScriptExtension(lowerBase) || isAlternativeExportArtifact(lowerRelative, lowerBase) || isBenignExtraFile(lowerBase) {
		return fileActionDrop, false, false
	}
	if hasHardRejectExtension(lowerBase) {
		return fileActionReject, false, false
	}
	if strings.HasSuffix(lowerBase, ".safetensors") {
		return fileActionKeep, false, true
	}
	return fileActionDrop, false, false
}
