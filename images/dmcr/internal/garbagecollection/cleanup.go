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
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/dynamic"
)

type AutoCleanupResult struct {
	Report         Report
	RegistryOutput string
}

var autoCleanupRunner = AutoCleanup

func Check(ctx context.Context, configPath string) (Report, error) {
	dynamicClient, err := NewInClusterDynamicClient()
	if err != nil {
		return Report{}, err
	}

	store, rootDirectory, err := newPrefixStoreFromConfig(configPath)
	if err != nil {
		return Report{}, err
	}

	return BuildReport(ctx, dynamicClient, store, rootDirectory)
}

func AutoCleanup(
	ctx context.Context,
	configPath string,
	registryBinary string,
	gcTimeout time.Duration,
) (AutoCleanupResult, error) {
	dynamicClient, err := NewInClusterDynamicClient()
	if err != nil {
		return AutoCleanupResult{}, err
	}

	store, rootDirectory, err := newPrefixStoreFromConfig(configPath)
	if err != nil {
		return AutoCleanupResult{}, err
	}

	report, err := BuildReport(ctx, dynamicClient, store, rootDirectory)
	if err != nil {
		return AutoCleanupResult{}, err
	}
	if err := deleteStalePrefixes(ctx, store, report); err != nil {
		return AutoCleanupResult{}, err
	}

	output, err := execGarbageCollect(ctx, Options{
		ConfigPath:      configPath,
		RegistryBinary:  registryBinary,
		GCTimeout:       gcTimeout,
		RescanInterval:  DefaultRescanInterval,
		ActivationDelay: DefaultActivationDelay,
	})
	if err != nil {
		return AutoCleanupResult{}, err
	}

	return AutoCleanupResult{
		Report:         report,
		RegistryOutput: strings.TrimSpace(string(output)),
	}, nil
}

func BuildReport(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	store prefixStore,
	rootDirectory string,
) (Report, error) {
	return buildReportWithClock(ctx, dynamicClient, store, rootDirectory, time.Now().UTC(), defaultDirectUploadOrphanStaleAge)
}

func buildReportWithClock(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	store prefixStore,
	rootDirectory string,
	now time.Time,
	directUploadStaleAge time.Duration,
) (Report, error) {
	live, err := DiscoverLivePrefixes(ctx, dynamicClient)
	if err != nil {
		return Report{}, err
	}

	storedRepositories, storedRawPrefixes, err := DiscoverStoredPrefixes(ctx, store, rootDirectory)
	if err != nil {
		return Report{}, err
	}

	report := buildReport(live, storedRepositories, storedRawPrefixes)

	directUploadInventory, err := discoverDirectUploadInventory(ctx, store, rootDirectory, now, directUploadStaleAge)
	if err != nil {
		return Report{}, err
	}
	report.StoredDirectUploadPrefixCount = directUploadInventory.StoredPrefixCount
	report.ReferencedDirectUploadPrefixCount = directUploadInventory.ReferencedPrefixCount
	report.StaleDirectUploadPrefixes = directUploadInventory.StalePrefixes
	return report, nil
}

func deleteStalePrefixes(ctx context.Context, store prefixStore, report Report) error {
	for _, entry := range report.StaleRepositories {
		if err := store.DeletePrefix(ctx, entry.Prefix); err != nil {
			return fmt.Errorf("delete stale repository prefix %s: %w", entry.Prefix, err)
		}
	}
	for _, entry := range report.StaleRawPrefixes {
		if err := store.DeletePrefix(ctx, entry.Prefix); err != nil {
			return fmt.Errorf("delete stale raw source mirror prefix %s: %w", entry.Prefix, err)
		}
	}
	for _, entry := range report.StaleDirectUploadPrefixes {
		if err := store.DeletePrefix(ctx, directUploadDeletePrefix(entry.Prefix)); err != nil {
			return fmt.Errorf("delete stale direct-upload prefix %s: %w", entry.Prefix, err)
		}
	}
	return nil
}

func newPrefixStoreFromConfig(configPath string) (prefixStore, string, error) {
	storageConfig, err := LoadStorageConfig(configPath)
	if err != nil {
		return nil, "", err
	}

	store, err := NewS3PrefixStore(storageConfig)
	if err != nil {
		return nil, "", err
	}
	return store, storageConfig.RootDirectory, nil
}

func directUploadDeletePrefix(prefix string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	if cleanPrefix == "" {
		return ""
	}
	return cleanPrefix + "/"
}
