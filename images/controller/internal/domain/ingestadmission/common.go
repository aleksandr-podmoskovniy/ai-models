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

package ingestadmission

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type OwnerBinding struct {
	Kind       string
	Name       string
	Namespace  string
	UID        string
	Generation int64
}

func ValidateOwnerBinding(owner OwnerBinding, identity publicationdata.Identity) error {
	switch {
	case strings.TrimSpace(owner.Kind) == "":
		return errors.New("publication owner kind must not be empty")
	case strings.TrimSpace(owner.Name) == "":
		return errors.New("publication owner name must not be empty")
	case strings.TrimSpace(owner.UID) == "":
		return errors.New("publication owner UID must not be empty")
	}
	if err := identity.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(owner.Name) != identity.Name {
		return fmt.Errorf("publication owner name %q does not match identity %q", owner.Name, identity.Name)
	}

	switch identity.Scope {
	case publicationdata.ScopeNamespaced:
		if strings.TrimSpace(owner.Namespace) == "" {
			return errors.New("publication owner namespace must not be empty for namespaced identity")
		}
		if strings.TrimSpace(owner.Namespace) != identity.Namespace {
			return fmt.Errorf(
				"publication owner namespace %q does not match identity namespace %q",
				owner.Namespace,
				identity.Namespace,
			)
		}
	case publicationdata.ScopeCluster:
		if strings.TrimSpace(owner.Namespace) != "" {
			return fmt.Errorf("cluster-scoped publication owner %q must not carry a namespace", owner.Name)
		}
	default:
		return fmt.Errorf("unsupported publication scope %q", identity.Scope)
	}

	return nil
}

func ValidateDeclaredInputFormat(format modelsv1alpha1.ModelInputFormat) error {
	switch format {
	case "", modelsv1alpha1.ModelInputFormatSafetensors, modelsv1alpha1.ModelInputFormatGGUF, modelsv1alpha1.ModelInputFormatDiffusers:
		return nil
	default:
		return fmt.Errorf("unsupported model input format %q", format)
	}
}

func ValidateRemoteFileName(fileName string, declaredFormat modelsv1alpha1.ModelInputFormat) error {
	normalized, err := normalizeFileName(fileName)
	if err != nil {
		return err
	}
	if err := ValidateDeclaredInputFormat(declaredFormat); err != nil {
		return err
	}
	if isArchiveFileName(normalized) {
		return nil
	}

	switch classifyDirectFileKind(normalized) {
	case directInputGGUF:
		if declaredFormat != "" && declaredFormat != modelsv1alpha1.ModelInputFormatGGUF {
			return fmt.Errorf("source file %q does not match declared input format %q", normalized, declaredFormat)
		}
	case directInputSafetensors:
		return errors.New("direct safetensors source requires an archive bundle with config.json and weights")
	}

	return nil
}

func normalizeFileName(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if trimmed == "" {
		return "", errors.New("source file name must not be empty")
	}

	base := strings.TrimSpace(filepath.Base(trimmed))
	switch base {
	case "", ".", "..", string(filepath.Separator):
		return "", errors.New("source file name must not be empty")
	}
	if strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("source file name %q must not be hidden", base)
	}
	return base, nil
}

func isArchiveFileName(fileName string) bool {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.zst") ||
		strings.HasSuffix(lower, ".tar.zstd") ||
		strings.HasSuffix(lower, ".tzst")
}

type directFileKind string

const (
	directInputUnknown     directFileKind = ""
	directInputGGUF        directFileKind = "gguf"
	directInputSafetensors directFileKind = "safetensors"
)

func classifyDirectFileKind(fileName string) directFileKind {
	lower := strings.ToLower(strings.TrimSpace(fileName))
	switch {
	case strings.HasSuffix(lower, ".gguf"):
		return directInputGGUF
	case strings.HasSuffix(lower, ".safetensors"):
		return directInputSafetensors
	default:
		return directInputUnknown
	}
}
