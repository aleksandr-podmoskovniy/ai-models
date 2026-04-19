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

type SourceAcquisitionMode string

const (
	SourceAcquisitionModeMirror SourceAcquisitionMode = "mirror"
	SourceAcquisitionModeDirect SourceAcquisitionMode = "direct"
)

func NormalizeSourceAcquisitionMode(mode SourceAcquisitionMode) SourceAcquisitionMode {
	normalized := SourceAcquisitionMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if normalized == "" {
		return SourceAcquisitionModeDirect
	}
	return normalized
}

func ValidateSourceAcquisitionMode(mode SourceAcquisitionMode) error {
	switch NormalizeSourceAcquisitionMode(mode) {
	case SourceAcquisitionModeMirror, SourceAcquisitionModeDirect:
		return nil
	default:
		return fmt.Errorf("unsupported source acquisition mode %q", strings.TrimSpace(string(mode)))
	}
}
