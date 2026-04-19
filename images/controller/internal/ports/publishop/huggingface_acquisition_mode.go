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

package publishop

import (
	"fmt"
	"strings"
)

type HuggingFaceAcquisitionMode string

const (
	HuggingFaceAcquisitionModeMirror HuggingFaceAcquisitionMode = "mirror"
	HuggingFaceAcquisitionModeDirect HuggingFaceAcquisitionMode = "direct"
)

func NormalizeHuggingFaceAcquisitionMode(mode HuggingFaceAcquisitionMode) HuggingFaceAcquisitionMode {
	normalized := HuggingFaceAcquisitionMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if normalized == "" {
		return HuggingFaceAcquisitionModeMirror
	}
	return normalized
}

func ValidateHuggingFaceAcquisitionMode(mode HuggingFaceAcquisitionMode) error {
	switch NormalizeHuggingFaceAcquisitionMode(mode) {
	case HuggingFaceAcquisitionModeMirror, HuggingFaceAcquisitionModeDirect:
		return nil
	default:
		return fmt.Errorf("unsupported huggingface acquisition mode %q", strings.TrimSpace(string(mode)))
	}
}
