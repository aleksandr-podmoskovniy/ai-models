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
	"os/exec"
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

func (a *Adapter) configure(ctx context.Context, configDir string, auth modelpackports.RegistryAuth) error {
	return a.run(ctx, configDir, auth, "version", "--show-update-notifications=false")
}

func (a *Adapter) login(ctx context.Context, configDir, reference string, auth modelpackports.RegistryAuth) error {
	if strings.TrimSpace(auth.Username) == "" {
		return errors.New("OCI username must not be empty")
	}
	if auth.Password == "" {
		return errors.New("OCI password must not be empty")
	}

	command := a.command(ctx, configDir, "login", registryFromOCIReference(reference), "-u", auth.Username, "--password-stdin")
	command.Args = append(command.Args, connectionFlags(auth)...)
	command.Env = runtimeEnvironment(configDir, auth)
	command.Stdin = strings.NewReader(auth.Password)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *Adapter) inspectRemote(ctx context.Context, _ string, reference string, auth modelpackports.RegistryAuth) (modelpackoci.InspectPayload, error) {
	return modelpackoci.InspectRemote(ctx, reference, auth)
}

func (a *Adapter) run(ctx context.Context, configDir string, auth modelpackports.RegistryAuth, args ...string) error {
	command := a.command(ctx, configDir, args...)
	command.Env = runtimeEnvironment(configDir, auth)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (a *Adapter) command(ctx context.Context, configDir string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, a.binaryPath(), append([]string{"--config", configDir, "--progress", "none", "--log-level", "error"}, args...)...)
	return command
}

func (a *Adapter) binaryPath() string {
	if strings.TrimSpace(a.BinaryPath) != "" {
		return a.BinaryPath
	}
	return defaultBinaryPath
}

func prepareContext(input modelpackports.PublishInput) (string, error) {
	modelDir := strings.TrimSpace(input.ModelDir)
	if modelDir == "" {
		return "", errors.New("model directory must not be empty")
	}

	contextDir, err := os.MkdirTemp("", "ai-model-kitops-kitfile-")
	if err != nil {
		return "", err
	}

	description := strings.TrimSpace(strings.ReplaceAll(input.Description, "\"", "'"))
	if description == "" {
		description = "Published model"
	}
	kitfile := strings.Join([]string{
		"manifestVersion: v1alpha2",
		"package:",
		fmt.Sprintf("  name: %s", packageName(input)),
		fmt.Sprintf("  description: \"%s\"", description),
		"model:",
		"  path: .",
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(contextDir, "Kitfile"), []byte(kitfile), 0o644); err != nil {
		os.RemoveAll(contextDir)
		return "", err
	}

	return contextDir, nil
}

func packageName(input modelpackports.PublishInput) string {
	if value := strings.TrimSpace(input.PackageName); value != "" {
		return value
	}
	return packageNameFromOCIReference(input.ArtifactURI)
}

func connectionFlags(auth modelpackports.RegistryAuth) []string {
	if auth.Insecure {
		return []string{"--tls-verify=false"}
	}
	return nil
}

func runtimeEnvironment(configDir string, auth modelpackports.RegistryAuth) []string {
	environment := os.Environ()
	environment = append(environment,
		"HOME="+configDir,
		"DOCKER_CONFIG="+filepath.Join(configDir, "docker"),
	)
	if strings.TrimSpace(auth.CAFile) != "" {
		environment = append(environment, "SSL_CERT_FILE="+auth.CAFile)
	}
	return environment
}

func registryFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(reference)
	withoutDigest := strings.SplitN(cleanReference, "@", 2)[0]
	registry, repository, found := strings.Cut(withoutDigest, "/")
	if !found || strings.TrimSpace(registry) == "" || strings.TrimSpace(repository) == "" {
		return ""
	}
	return registry
}

func immutableOCIReference(reference, digest string) string {
	cleanReference := strings.TrimSpace(reference)
	cleanDigest := strings.TrimSpace(digest)
	if cleanReference == "" || cleanDigest == "" {
		return ""
	}

	withoutDigest := strings.SplitN(cleanReference, "@", 2)[0]
	repositoryPart := withoutDigest[strings.LastIndex(withoutDigest, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		withoutDigest = withoutDigest[:strings.LastIndex(withoutDigest, ":")]
	}

	return withoutDigest + "@" + cleanDigest
}

func packageNameFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	if index := strings.LastIndex(cleanReference, "/"); index >= 0 {
		cleanReference = cleanReference[index+1:]
	}
	if index := strings.LastIndex(cleanReference, ":"); index >= 0 {
		cleanReference = cleanReference[:index]
	}
	if cleanReference == "" {
		return "model"
	}
	return cleanReference
}
