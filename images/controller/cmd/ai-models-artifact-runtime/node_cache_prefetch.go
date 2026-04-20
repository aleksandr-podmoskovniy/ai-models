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

package main

import (
	"context"

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type nodeDesiredArtifactLoader struct {
	client   *k8snodecacheruntime.Client
	nodeName string
}

func (l nodeDesiredArtifactLoader) LoadDesiredArtifacts(ctx context.Context) ([]nodecache.DesiredArtifact, error) {
	return l.client.LoadNodeDesiredArtifacts(ctx, l.nodeName)
}

func nodeCachePrefetcher(auth modelpackports.RegistryAuth) func(context.Context, nodecache.DesiredArtifact, string) error {
	materializer := modelpackoci.NewMaterializer()
	return func(ctx context.Context, artifact nodecache.DesiredArtifact, destinationDir string) error {
		_, err := materializer.Materialize(ctx, modelpackports.MaterializeInput{
			ArtifactURI:    artifact.ArtifactURI,
			ArtifactDigest: artifact.Digest,
			DestinationDir: destinationDir,
			ArtifactFamily: artifact.Family,
		}, auth)
		return err
	}
}
