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

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
)

type PrefetchFunc func(context.Context, intentcontract.ArtifactIntent, string) error

func EnsureDesiredArtifacts(
	ctx context.Context,
	cacheRoot string,
	intents []intentcontract.ArtifactIntent,
	run PrefetchFunc,
) error {
	if run == nil {
		return errors.New("node cache prefetch function must not be nil")
	}
	cacheRoot = strings.TrimSpace(cacheRoot)
	if cacheRoot == "" {
		return errors.New("node cache prefetch cache-root must not be empty")
	}
	intents, err := intentcontract.NormalizeIntents(intents)
	if err != nil || len(intents) == 0 {
		return err
	}

	snapshot, err := Scan(cacheRoot)
	if err != nil {
		return err
	}
	ready := readyDigestSet(snapshot)

	for _, intent := range intents {
		if _, found := ready[intent.Digest]; found {
			continue
		}
		destinationDir := StorePath(cacheRoot, intent.Digest)
		slog.Default().Info(
			"node cache prefetch started",
			slog.String("artifactURI", intent.ArtifactURI),
			slog.String("digest", intent.Digest),
			slog.String("destinationDir", destinationDir),
		)
		if err := run(ctx, intent, destinationDir); err != nil {
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
