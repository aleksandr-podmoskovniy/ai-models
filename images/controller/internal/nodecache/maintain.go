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
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MaintenanceOptions struct {
	CacheRoot         string
	MaxTotalSizeBytes int64
	MaxUnusedAge      time.Duration
	ScanInterval      time.Duration
}

type MaintenanceResult struct {
	Snapshot Snapshot
	Plan     EvictionPlan
}

func NormalizeMaintenanceOptions(options MaintenanceOptions) MaintenanceOptions {
	options.CacheRoot = filepath.Clean(strings.TrimSpace(options.CacheRoot))
	if options.ScanInterval == 0 {
		options.ScanInterval = DefaultMaintenancePeriod
	}
	return options
}

func ValidateMaintenanceOptions(options MaintenanceOptions) error {
	options = NormalizeMaintenanceOptions(options)
	switch {
	case strings.TrimSpace(options.CacheRoot) == "" || options.CacheRoot == ".":
		return errors.New("node cache maintenance cache-root must not be empty")
	case options.MaxTotalSizeBytes < 0:
		return errors.New("node cache maintenance max total size must not be negative")
	case options.MaxUnusedAge < 0:
		return errors.New("node cache maintenance max unused age must not be negative")
	case options.ScanInterval <= 0:
		return errors.New("node cache maintenance scan interval must be greater than zero")
	default:
		return nil
	}
}

func MaintainOnce(options MaintenanceOptions) (MaintenanceResult, error) {
	options = NormalizeMaintenanceOptions(options)
	if err := ValidateMaintenanceOptions(options); err != nil {
		return MaintenanceResult{}, err
	}

	snapshot, err := Scan(options.CacheRoot)
	if err != nil {
		return MaintenanceResult{}, err
	}
	return maintainSnapshot(snapshot, PlanInput{
		MaxTotalSizeBytes: options.MaxTotalSizeBytes,
		MaxUnusedAge:      options.MaxUnusedAge,
	})
}

func maintainSnapshot(snapshot Snapshot, input PlanInput) (MaintenanceResult, error) {
	plan := PlanEviction(snapshot, input)
	if err := applyEvictionPlan(plan); err != nil {
		return MaintenanceResult{}, err
	}
	return MaintenanceResult{Snapshot: snapshot, Plan: plan}, nil
}

func applyEvictionPlan(plan EvictionPlan) error {
	for _, candidate := range plan.Candidates {
		if err := os.RemoveAll(candidate.DestinationDir); err != nil {
			return err
		}
		slog.Default().Info(
			"node cache entry evicted",
			slog.String("destinationDir", candidate.DestinationDir),
			slog.String("artifactDigest", candidate.Digest),
			slog.String("reason", string(candidate.Reason)),
			slog.Int64("reclaimBytes", candidate.ReclaimBytes),
		)
	}
	if len(plan.Candidates) > 0 {
		slog.Default().Info(
			"node cache maintenance completed",
			slog.Int("evictedEntries", len(plan.Candidates)),
			slog.Int64("reclaimBytes", plan.ReclaimBytes),
			slog.Int64("residualBytes", plan.ResidualSizeBytes),
		)
	}
	return nil
}
