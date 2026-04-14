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
	"os/exec"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

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
	return exec.CommandContext(ctx, a.binaryPath(), append([]string{"--config", configDir, "--progress", "none", "--log-level", "error"}, args...)...)
}

func (a *Adapter) binaryPath() string {
	if strings.TrimSpace(a.BinaryPath) != "" {
		return a.BinaryPath
	}
	return defaultBinaryPath
}
