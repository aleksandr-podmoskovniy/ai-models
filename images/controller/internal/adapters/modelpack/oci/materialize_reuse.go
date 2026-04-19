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

package oci

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func maybeReuseMaterialization(destination, digest string) (modelpackports.MaterializeResult, bool, error) {
	marker, err := nodecache.ReadMarker(destination)
	if err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	if marker == nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if strings.TrimSpace(marker.Digest) != strings.TrimSpace(digest) {
		return modelpackports.MaterializeResult{}, false, nil
	}
	modelPath := strings.TrimSpace(marker.ModelPath)
	contractPath := modelpackports.MaterializedModelPath(destination)
	if _, err := os.Stat(contractPath); err == nil {
		modelPath = contractPath
	}
	if strings.TrimSpace(modelPath) == "" {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if _, err := os.Stat(modelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	return modelpackports.MaterializeResult{
		ModelPath:  modelPath,
		MarkerPath: filepath.Join(destination, nodecache.MarkerFileName),
	}, true, nil
}
