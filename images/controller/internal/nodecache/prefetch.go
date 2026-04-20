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

package nodecache

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

type PrefetchFunc func(context.Context, DesiredArtifact, string) error

func EnsureDesiredArtifacts(
	ctx context.Context,
	cacheRoot string,
	artifacts []DesiredArtifact,
	run PrefetchFunc,
) error {
	if run == nil {
		return errors.New("node cache prefetch function must not be nil")
	}
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		return errors.New("node cache prefetch cache-root must not be empty")
	}
	artifacts, err := NormalizeDesiredArtifacts(artifacts)
	if err != nil || len(artifacts) == 0 {
		return err
	}

	snapshot, err := Scan(cacheRoot)
	if err != nil {
		return err
	}
	ready := readyDigestSet(snapshot)

	for _, artifact := range artifacts {
		if _, found := ready[artifact.Digest]; found {
			continue
		}
		destinationDir := StorePath(cacheRoot, artifact.Digest)
		slog.Default().Info(
			"node cache prefetch started",
			slog.String("artifactURI", artifact.ArtifactURI),
			slog.String("digest", artifact.Digest),
			slog.String("destinationDir", destinationDir),
		)
		if err := run(ctx, artifact, destinationDir); err != nil {
			return err
		}
	}
	return nil
}

func readyDigestSet(snapshot Snapshot) map[string]struct{} {
	ready := make(map[string]struct{}, len(snapshot.Entries))
	for _, entry := range snapshot.Entries {
		if !entry.Ready {
			continue
		}
		ready[entry.Digest] = struct{}{}
	}
	return ready
}
