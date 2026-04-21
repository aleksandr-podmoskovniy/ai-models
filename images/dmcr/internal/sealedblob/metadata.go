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

package sealedblob

import (
	"encoding/json"
	"errors"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

const (
	MetadataSuffix  = ".dmcr-sealed"
	MetadataVersion = "dmcr-sealed-blob-v1"
)

type Metadata struct {
	Version      string `json:"version"`
	Digest       string `json:"digest"`
	PhysicalPath string `json:"physicalPath"`
	SizeBytes    int64  `json:"sizeBytes"`
}

func MetadataPath(blobDataPath string) string {
	return strings.TrimSpace(blobDataPath) + MetadataSuffix
}

func IsMetadataPath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), MetadataSuffix)
}

func CanonicalPathFromMetadataPath(path string) (string, bool) {
	cleanPath := strings.TrimSpace(path)
	if !IsMetadataPath(cleanPath) {
		return "", false
	}
	return strings.TrimSuffix(cleanPath, MetadataSuffix), true
}

func LooksLikeCanonicalBlobDataPath(path string) bool {
	cleanPath := strings.TrimSpace(path)
	return strings.HasSuffix(cleanPath, "/data") && strings.Contains(cleanPath, "/docker/registry/v2/blobs/")
}

func DigestFromCanonicalBlobDataPath(path string) (string, bool) {
	cleanPath := strings.Trim(strings.TrimSpace(path), "/")
	const marker = "docker/registry/v2/blobs/"
	index := strings.Index(cleanPath, marker)
	if index < 0 {
		return "", false
	}
	relativePath := cleanPath[index+len(marker):]
	parts := strings.Split(relativePath, "/")
	if len(parts) < 4 {
		return "", false
	}
	algorithm := strings.TrimSpace(parts[0])
	prefix := strings.TrimSpace(parts[1])
	encoded := strings.TrimSpace(parts[2])
	if algorithm == "" || prefix == "" || encoded == "" || parts[3] != "data" {
		return "", false
	}
	if len(prefix) != 2 || !strings.HasPrefix(encoded, prefix) {
		return "", false
	}
	resolvedDigest := algorithm + ":" + encoded
	if _, err := digest.Parse(resolvedDigest); err != nil {
		return "", false
	}
	return resolvedDigest, true
}

func Marshal(metadata Metadata) ([]byte, error) {
	if err := validateMetadata(metadata); err != nil {
		return nil, err
	}
	return json.Marshal(metadata)
}

func Unmarshal(payload []byte) (Metadata, error) {
	var metadata Metadata
	if err := json.Unmarshal(payload, &metadata); err != nil {
		return Metadata{}, err
	}
	if err := validateMetadata(metadata); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

func validateMetadata(metadata Metadata) error {
	switch {
	case strings.TrimSpace(metadata.Version) != MetadataVersion:
		return errors.New("sealed blob metadata version must match")
	case strings.TrimSpace(metadata.Digest) == "":
		return errors.New("sealed blob metadata digest must not be empty")
	case strings.TrimSpace(metadata.PhysicalPath) == "":
		return errors.New("sealed blob metadata physical path must not be empty")
	case metadata.SizeBytes < 0:
		return errors.New("sealed blob metadata size must not be negative")
	default:
		return nil
	}
}
