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

package sourcemirror

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

var ErrSnapshotNotFound = errors.New("source mirror snapshot not found")

type SnapshotLocator struct {
	Provider string `json:"provider"`
	Subject  string `json:"subject"`
	Revision string `json:"revision"`
}

type SnapshotFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
	ETag      string `json:"etag,omitempty"`
}

type SnapshotManifest struct {
	Locator   SnapshotLocator `json:"locator"`
	Files     []SnapshotFile  `json:"files"`
	CreatedAt time.Time       `json:"createdAt,omitempty"`
}

// +kubebuilder:validation:Enum=Pending;Downloading;Completed;Failed
type SnapshotPhase string

const (
	SnapshotPhasePending     SnapshotPhase = "Pending"
	SnapshotPhaseDownloading SnapshotPhase = "Downloading"
	SnapshotPhaseCompleted   SnapshotPhase = "Completed"
	SnapshotPhaseFailed      SnapshotPhase = "Failed"
)

type SnapshotFileState struct {
	Path              string                             `json:"path"`
	Phase             SnapshotPhase                      `json:"phase"`
	BytesConfirmed    int64                              `json:"bytesConfirmed,omitempty"`
	MultipartUploadID string                             `json:"multipartUploadID,omitempty"`
	CompletedParts    []uploadstagingports.CompletedPart `json:"completedParts,omitempty"`
	LastError         string                             `json:"lastError,omitempty"`
	UpdatedAt         time.Time                          `json:"updatedAt,omitempty"`
}

type SnapshotState struct {
	Locator   SnapshotLocator     `json:"locator"`
	Phase     SnapshotPhase       `json:"phase"`
	Files     []SnapshotFileState `json:"files,omitempty"`
	UpdatedAt time.Time           `json:"updatedAt,omitempty"`
}

type Store interface {
	SaveManifest(ctx context.Context, manifest SnapshotManifest) error
	LoadManifest(ctx context.Context, locator SnapshotLocator) (SnapshotManifest, error)
	SaveState(ctx context.Context, state SnapshotState) error
	LoadState(ctx context.Context, locator SnapshotLocator) (SnapshotState, error)
	DeleteSnapshot(ctx context.Context, locator SnapshotLocator) error
}

func SnapshotPrefix(basePrefix string, locator SnapshotLocator) string {
	segments := []string{strings.Trim(strings.TrimSpace(basePrefix), "/")}
	segments = append(segments, strings.ToLower(strings.TrimSpace(locator.Provider)))
	segments = append(segments, splitPathSegments(locator.Subject)...)
	segments = append(segments, splitPathSegments(locator.Revision)...)
	return path.Join(segments...)
}

func SnapshotFilesPrefix(snapshotPrefix string) string {
	return path.Join(strings.Trim(strings.TrimSpace(snapshotPrefix), "/"), "files")
}

func SnapshotFileObjectKey(snapshotPrefix, relativePath string) string {
	cleanRelative := path.Clean(strings.Trim(strings.TrimSpace(strings.ReplaceAll(relativePath, "\\", "/")), "/"))
	return path.Join(SnapshotFilesPrefix(snapshotPrefix), cleanRelative)
}

func (l SnapshotLocator) Validate() error {
	if strings.TrimSpace(l.Provider) == "" {
		return errors.New("source mirror provider must not be empty")
	}
	if err := validatePathLikeIdentifier(strings.TrimSpace(l.Provider), false, "provider"); err != nil {
		return err
	}
	if strings.TrimSpace(l.Subject) == "" {
		return errors.New("source mirror subject must not be empty")
	}
	if err := validatePathLikeIdentifier(strings.TrimSpace(l.Subject), true, "subject"); err != nil {
		return err
	}
	if strings.TrimSpace(l.Revision) == "" {
		return errors.New("source mirror revision must not be empty")
	}
	if err := validatePathLikeIdentifier(strings.TrimSpace(l.Revision), true, "revision"); err != nil {
		return err
	}
	return nil
}

func (m SnapshotManifest) Validate() error {
	if err := m.Locator.Validate(); err != nil {
		return err
	}
	if len(m.Files) == 0 {
		return errors.New("source mirror manifest files must not be empty")
	}
	seen := make(map[string]struct{}, len(m.Files))
	for index, file := range m.Files {
		cleanPath, err := validateRelativeFilePath(file.Path, "manifest.files", index)
		if err != nil {
			return err
		}
		if _, exists := seen[cleanPath]; exists {
			return fmt.Errorf("source mirror manifest files[%d] duplicates path %q", index, cleanPath)
		}
		seen[cleanPath] = struct{}{}
	}
	return nil
}

func (s SnapshotState) Validate() error {
	if err := s.Locator.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(string(s.Phase)) == "" {
		return errors.New("source mirror snapshot phase must not be empty")
	}
	for index, file := range s.Files {
		if _, err := validateRelativeFilePath(file.Path, "state.files", index); err != nil {
			return err
		}
		if strings.TrimSpace(string(file.Phase)) == "" {
			return fmt.Errorf("source mirror state.files[%d] phase must not be empty", index)
		}
		if file.BytesConfirmed < 0 {
			return fmt.Errorf("source mirror state.files[%d] bytesConfirmed must not be negative", index)
		}
	}
	return nil
}

func validateRelativeFilePath(rawPath, field string, index int) (string, error) {
	clean := path.Clean(strings.TrimSpace(strings.ReplaceAll(rawPath, "\\", "/")))
	switch {
	case clean == "", clean == ".":
		return "", fmt.Errorf("source mirror %s[%d] path must not be empty", field, index)
	case strings.HasPrefix(clean, "/"):
		return "", fmt.Errorf("source mirror %s[%d] path must be relative, got %q", field, index, rawPath)
	case strings.HasPrefix(clean, "../") || clean == "..":
		return "", fmt.Errorf("source mirror %s[%d] path must not escape the snapshot root, got %q", field, index, rawPath)
	default:
		return clean, nil
	}
}

func validatePathLikeIdentifier(value string, allowSlash bool, field string) error {
	parts := []string{value}
	if allowSlash {
		parts = strings.Split(value, "/")
	}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		switch trimmed {
		case "", ".", "..":
			return fmt.Errorf("source mirror %s contains invalid path segment %q", field, part)
		}
	}
	return nil
}

func splitPathSegments(raw string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(raw), "/"), "/")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
