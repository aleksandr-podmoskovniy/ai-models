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

package garbagecollection

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

func execGarbageCollect(ctx context.Context, options Options) ([]byte, error) {
	gcContext, cancel := context.WithTimeout(ctx, options.GCTimeout)
	defer cancel()

	command := exec.CommandContext(gcContext, options.RegistryBinary, "garbage-collect", options.ConfigPath, "--delete-untagged")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run dmcr garbage-collect: %s", boundedCommandOutputMessage(string(output), err.Error()))
	}
	return output, nil
}

func boundedCommandOutputMessage(output, fallback string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return strings.TrimSpace(fallback)
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= 8 {
		return output
	}

	sum := sha256.Sum256([]byte(output))
	return fmt.Sprintf(
		"output_line_count=%d output_sha256=%s first_lines=%q last_lines=%q",
		len(lines),
		hex.EncodeToString(sum[:]),
		strings.Join(lines[:2], "\n"),
		strings.Join(lines[len(lines)-2:], "\n"),
	)
}
