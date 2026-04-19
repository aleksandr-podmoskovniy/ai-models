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

	k8snodecacheintent "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheintent"
	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type nodeIntentLoader struct {
	client   *k8snodecacheintent.Client
	nodeName string
}

func (l nodeIntentLoader) LoadIntents(ctx context.Context) ([]intentcontract.ArtifactIntent, error) {
	return l.client.LoadNodeIntents(ctx, l.nodeName)
}

func nodeCachePrefetcher(auth modelpackports.RegistryAuth) func(context.Context, intentcontract.ArtifactIntent, string) error {
	materializer := modelpackoci.NewMaterializer()
	return func(ctx context.Context, intent intentcontract.ArtifactIntent, destinationDir string) error {
		_, err := materializer.Materialize(ctx, modelpackports.MaterializeInput{
			ArtifactURI:    intent.ArtifactURI,
			ArtifactDigest: intent.Digest,
			DestinationDir: destinationDir,
			ArtifactFamily: intent.Family,
		}, auth)
		return err
	}
}
