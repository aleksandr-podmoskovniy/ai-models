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

package kitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const defaultBinaryPath = "/usr/local/bin/kit"

type Adapter struct {
	BinaryPath string
}

func New() *Adapter {
	return &Adapter{BinaryPath: defaultBinaryPath}
}

func (a *Adapter) Publish(ctx context.Context, input modelpackports.PublishInput, auth modelpackports.RegistryAuth) (modelpackports.PublishResult, error) {
	if a == nil {
		return modelpackports.PublishResult{}, errors.New("kitops adapter must not be nil")
	}
	if strings.TrimSpace(input.ModelDir) == "" {
		return modelpackports.PublishResult{}, errors.New("model directory must not be empty")
	}
	if strings.TrimSpace(input.ArtifactURI) == "" {
		return modelpackports.PublishResult{}, errors.New("artifact URI must not be empty")
	}

	configDir, err := os.MkdirTemp("", "ai-model-kitops-config-")
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	defer os.RemoveAll(configDir)

	contextDir, err := prepareContext(input)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	defer os.RemoveAll(contextDir)

	if err := a.configure(ctx, configDir, auth); err != nil {
		return modelpackports.PublishResult{}, err
	}
	if err := a.login(ctx, configDir, input.ArtifactURI, auth); err != nil {
		return modelpackports.PublishResult{}, err
	}
	if err := a.run(ctx, configDir, auth, "pack", input.ModelDir, "-f", filepath.Join(contextDir, "Kitfile"), "-t", input.ArtifactURI, "--use-model-pack"); err != nil {
		return modelpackports.PublishResult{}, fmt.Errorf("failed to pack ModelPack: %w", err)
	}
	pushArgs := append([]string{"push", input.ArtifactURI}, connectionFlags(auth)...)
	if err := a.run(ctx, configDir, auth, pushArgs...); err != nil {
		return modelpackports.PublishResult{}, fmt.Errorf("failed to push ModelPack: %w", err)
	}

	inspectPayload, err := a.inspectRemote(ctx, configDir, input.ArtifactURI, auth)
	if err != nil {
		return modelpackports.PublishResult{}, err
	}
	if err := modelpackoci.ValidatePayload(inspectPayload); err != nil {
		return modelpackports.PublishResult{}, err
	}

	digest := modelpackoci.ArtifactDigest(inspectPayload)
	if strings.TrimSpace(digest) == "" {
		return modelpackports.PublishResult{}, errors.New("kitops inspect payload is missing digest")
	}

	return modelpackports.PublishResult{
		Reference: immutableOCIReference(input.ArtifactURI, digest),
		Digest:    digest,
		MediaType: modelpackoci.ArtifactMediaType(inspectPayload),
		SizeBytes: modelpackoci.InspectSizeBytes(inspectPayload),
	}, nil
}

func (a *Adapter) Remove(ctx context.Context, reference string, auth modelpackports.RegistryAuth) error {
	if a == nil {
		return errors.New("kitops adapter must not be nil")
	}
	if strings.TrimSpace(reference) == "" {
		return errors.New("cleanup reference must not be empty")
	}

	configDir, err := os.MkdirTemp("", "ai-model-kitops-cleanup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir)

	if err := a.configure(ctx, configDir, auth); err != nil {
		return err
	}
	if err := a.login(ctx, configDir, reference, auth); err != nil {
		return err
	}
	removeArgs := append([]string{"remove", "--remote", reference}, connectionFlags(auth)...)
	if err := a.run(ctx, configDir, auth, removeArgs...); err != nil {
		return fmt.Errorf("failed to remove remote ModelPack: %w", err)
	}

	return nil
}

func (a *Adapter) inspectRemote(ctx context.Context, _ string, reference string, auth modelpackports.RegistryAuth) (modelpackoci.InspectPayload, error) {
	return modelpackoci.InspectRemote(ctx, reference, auth)
}
