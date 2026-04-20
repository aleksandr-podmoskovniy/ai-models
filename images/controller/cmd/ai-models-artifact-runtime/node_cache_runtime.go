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
	"fmt"
	"log/slog"
	"strings"
	"time"

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	nodeCacheRootEnv         = nodecache.RuntimeCacheRootEnv
	nodeCacheMaxSizeEnv      = nodecache.RuntimeMaxSizeEnv
	nodeCacheMaxUnusedAgeEnv = nodecache.RuntimeMaxUnusedAgeEnv
	nodeCacheScanIntervalEnv = nodecache.RuntimeScanIntervalEnv
	nodeCacheNodeNameEnv     = k8snodecacheruntime.RuntimeNodeNameEnv
)

type nodeCacheRuntimeConfig struct {
	CacheRoot    string
	MaxTotalSize string
	MaxUnusedAge time.Duration
	ScanInterval time.Duration
	NodeName     string
}

func runNodeCacheRuntime(args []string) int {
	config, exitCode, err := parseNodeCacheRuntimeConfig(args)
	if err != nil {
		return cmdsupport.CommandError(commandNodeCacheRuntime, err)
	}
	if exitCode != 0 {
		return exitCode
	}

	maxTotalSizeBytes, err := parseNodeCacheRuntimeSize(config.MaxTotalSize)
	if err != nil {
		return cmdsupport.CommandError(commandNodeCacheRuntime, err)
	}

	logger := slog.Default().With(
		slog.String("cacheRoot", strings.TrimSpace(config.CacheRoot)),
		slog.String("maxTotalSize", strings.TrimSpace(config.MaxTotalSize)),
		slog.String("maxUnusedAge", config.MaxUnusedAge.String()),
		slog.String("scanInterval", config.ScanInterval.String()),
		slog.String("nodeName", strings.TrimSpace(config.NodeName)),
	)
	logger.Info("node cache runtime started")

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	desiredArtifactsClient, err := k8snodecacheruntime.NewInClusterDesiredArtifactsClient()
	if err != nil {
		return cmdsupport.CommandError(commandNodeCacheRuntime, err)
	}
	if err := nodecache.RunRuntimeLoop(ctx, nodecache.RuntimeOptions{
		Maintenance: nodecache.MaintenanceOptions{
			CacheRoot:         config.CacheRoot,
			MaxTotalSizeBytes: maxTotalSizeBytes,
			MaxUnusedAge:      config.MaxUnusedAge,
			ScanInterval:      config.ScanInterval,
		},
	}, nodeDesiredArtifactLoader{client: desiredArtifactsClient, nodeName: config.NodeName}, nodeCachePrefetcher(cmdsupport.RegistryAuthFromEnv(publicationOCIInsecureEnv))); err != nil {
		return cmdsupport.CommandError(commandNodeCacheRuntime, err)
	}

	return 0
}

func parseNodeCacheRuntimeConfig(args []string) (nodeCacheRuntimeConfig, int, error) {
	config := nodeCacheRuntimeConfig{
		CacheRoot:    cmdsupport.EnvOr(nodeCacheRootEnv, ""),
		MaxTotalSize: cmdsupport.EnvOr(nodeCacheMaxSizeEnv, ""),
		MaxUnusedAge: durationEnvOr(nodeCacheMaxUnusedAgeEnv, nodecache.DefaultMaxUnusedAge),
		ScanInterval: durationEnvOr(nodeCacheScanIntervalEnv, nodecache.DefaultMaintenancePeriod),
		NodeName:     cmdsupport.EnvOr(nodeCacheNodeNameEnv, ""),
	}

	flags := cmdsupport.NewFlagSet(commandNodeCacheRuntime)
	flags.StringVar(&config.CacheRoot, "cache-root", config.CacheRoot, "Shared node-local cache root.")
	flags.StringVar(&config.MaxTotalSize, "max-total-size", config.MaxTotalSize, "Maximum total cache size before size-pressure eviction.")
	flags.DurationVar(&config.MaxUnusedAge, "max-unused-age", config.MaxUnusedAge, "Maximum age since last use before idle eviction.")
	flags.DurationVar(&config.ScanInterval, "scan-interval", config.ScanInterval, "Maintenance scan interval.")
	flags.StringVar(&config.NodeName, "node-name", config.NodeName, "Current node name used to resolve the required published artifacts for this node.")
	if err := flags.Parse(args); err != nil {
		return nodeCacheRuntimeConfig{}, 2, err
	}
	if strings.TrimSpace(config.CacheRoot) == "" {
		return nodeCacheRuntimeConfig{}, 2, fmt.Errorf("%s must not be empty", nodeCacheRootEnv)
	}
	if strings.TrimSpace(config.NodeName) == "" {
		return nodeCacheRuntimeConfig{}, 2, fmt.Errorf("%s must not be empty", nodeCacheNodeNameEnv)
	}
	return config, 0, nil
}

func parseNodeCacheRuntimeSize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	quantity, err := resource.ParseQuantity(value)
	if err != nil {
		return 0, fmt.Errorf("parse node cache max total size: %w", err)
	}
	return quantity.Value(), nil
}

func durationEnvOr(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(cmdsupport.EnvOr(name, ""))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
