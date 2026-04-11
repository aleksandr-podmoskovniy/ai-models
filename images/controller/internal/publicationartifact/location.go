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

package publicationartifact

import (
	"errors"
	"fmt"
	"strings"

	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"k8s.io/apimachinery/pkg/types"
)

// BuildOCIArtifactReference returns the controller-owned destination tag for the
// current OCI-backed publication implementation.
func BuildOCIArtifactReference(root string, identity publication.Identity, uid types.UID) (string, error) {
	if err := identity.Validate(); err != nil {
		return "", err
	}

	root = strings.Trim(strings.TrimSpace(root), "/")
	if root == "" {
		return "", errors.New("publication artifact OCI root must not be empty")
	}
	if strings.Contains(root, "://") {
		return "", fmt.Errorf("publication artifact OCI root must not use a URL scheme, got %q", root)
	}

	uidValue := strings.TrimSpace(string(uid))
	if uidValue == "" {
		return "", errors.New("publication artifact UID must not be empty")
	}

	segments := []string{root, "catalog"}
	switch identity.Scope {
	case publication.ScopeNamespaced:
		segments = append(segments, "namespaced", identity.Namespace, identity.Name)
	case publication.ScopeCluster:
		segments = append(segments, "cluster", identity.Name)
	default:
		return "", fmt.Errorf("unsupported publication scope %q", identity.Scope)
	}
	segments = append(segments, uidValue)

	return strings.Join(segments, "/") + ":published", nil
}
