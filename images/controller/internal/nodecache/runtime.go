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
	"time"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
)

type IntentLoader interface {
	LoadIntents(context.Context) ([]intentcontract.ArtifactIntent, error)
}

type RuntimeOptions struct {
	Maintenance MaintenanceOptions
}

func RunRuntimeLoop(ctx context.Context, options RuntimeOptions, loader IntentLoader, prefetch PrefetchFunc) error {
	if loader == nil {
		return errors.New("node cache runtime intent loader must not be nil")
	}
	options.Maintenance = NormalizeMaintenanceOptions(options.Maintenance)
	if err := ValidateMaintenanceOptions(options.Maintenance); err != nil {
		return err
	}
	if err := runRuntimeCycle(ctx, options, loader, prefetch); err != nil {
		return err
	}

	ticker := time.NewTicker(options.Maintenance.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runRuntimeCycle(ctx, options, loader, prefetch); err != nil {
				return err
			}
		}
	}
}

func runRuntimeCycle(ctx context.Context, options RuntimeOptions, loader IntentLoader, prefetch PrefetchFunc) error {
	intents, err := loader.LoadIntents(ctx)
	if err != nil {
		return err
	}
	if err := EnsureDesiredArtifacts(ctx, options.Maintenance.CacheRoot, intents, prefetch); err != nil {
		return err
	}

	snapshot, err := Scan(options.Maintenance.CacheRoot)
	if err != nil {
		return err
	}
	_, err = maintainSnapshot(snapshot, PlanInput{
		MaxTotalSizeBytes: options.Maintenance.MaxTotalSizeBytes,
		MaxUnusedAge:      options.Maintenance.MaxUnusedAge,
		ProtectedDigests:  intentcontract.ProtectedDigests(intents),
	})
	return err
}
