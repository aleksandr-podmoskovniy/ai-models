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

package modelpack

import (
	"context"
	"path/filepath"
	"strings"
)

const MaterializedModelPathName = "model"

type RegistryAuth struct {
	Username string
	Password string
	CAFile   string
	Insecure bool
}

type PublishInput struct {
	ModelDir    string
	ArtifactURI string
}

type PublishResult struct {
	Reference string
	Digest    string
	MediaType string
	SizeBytes int64
}

type MaterializeInput struct {
	ArtifactURI    string
	ArtifactDigest string
	DestinationDir string
	ArtifactFamily string
}

type MaterializeResult struct {
	ModelPath  string
	Digest     string
	MediaType  string
	MarkerPath string
}

type Publisher interface {
	Publish(ctx context.Context, input PublishInput, auth RegistryAuth) (PublishResult, error)
}

type Remover interface {
	Remove(ctx context.Context, reference string, auth RegistryAuth) error
}

type Materializer interface {
	Materialize(ctx context.Context, input MaterializeInput, auth RegistryAuth) (MaterializeResult, error)
}

func MaterializedModelPath(destinationDir string) string {
	destinationDir = filepath.Clean(strings.TrimSpace(destinationDir))
	if destinationDir == "" || destinationDir == "." {
		return MaterializedModelPathName
	}

	return filepath.Join(destinationDir, MaterializedModelPathName)
}
