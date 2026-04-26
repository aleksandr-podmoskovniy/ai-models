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
)

type DesiredArtifactLoader interface {
	LoadDesiredArtifacts(context.Context) ([]DesiredArtifact, error)
}

type RuntimeOptions struct {
	Maintenance MaintenanceOptions
}

func RunRuntimeLoop(ctx context.Context, options RuntimeOptions, loader DesiredArtifactLoader, prefetch PrefetchFunc) error {
	if loader == nil {
		return errors.New("node cache runtime desired artifact loader must not be nil")
	}
	options.Maintenance = NormalizeMaintenanceOptions(options.Maintenance)
	if err := ValidateMaintenanceOptions(options.Maintenance); err != nil {
		return err
	}
	retryState := NewPrefetchRetryState(PrefetchRetryOptions{})
	if err := runRuntimeCycleWithRetry(ctx, options, loader, prefetch, retryState); err != nil {
		return err
	}

	ticker := time.NewTicker(options.Maintenance.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := runRuntimeCycleWithRetry(ctx, options, loader, prefetch, retryState); err != nil {
				return err
			}
		}
	}
}

func runRuntimeCycle(ctx context.Context, options RuntimeOptions, loader DesiredArtifactLoader, prefetch PrefetchFunc) error {
	return runRuntimeCycleWithRetry(ctx, options, loader, prefetch, NewPrefetchRetryState(PrefetchRetryOptions{}))
}

func runRuntimeCycleWithRetry(ctx context.Context, options RuntimeOptions, loader DesiredArtifactLoader, prefetch PrefetchFunc, retryState *PrefetchRetryState) error {
	artifacts, err := loader.LoadDesiredArtifacts(ctx)
	if err != nil {
		return err
	}
	if err := EnsureDesiredArtifactsWithRetry(ctx, options.Maintenance.CacheRoot, artifacts, prefetch, retryState); err != nil {
		return err
	}

	snapshot, err := Scan(options.Maintenance.CacheRoot)
	if err != nil {
		return err
	}
	_, err = maintainSnapshot(snapshot, PlanInput{
		MaxTotalSizeBytes: options.Maintenance.MaxTotalSizeBytes,
		MaxUnusedAge:      options.Maintenance.MaxUnusedAge,
		ProtectedDigests:  ProtectedDigests(artifacts),
	})
	return err
}
