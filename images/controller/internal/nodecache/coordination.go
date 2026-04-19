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
	"os"
	"path/filepath"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	CoordinationModeShared = "shared-cache"

	coordinationLockDirName    = ".coordination"
	coordinationHeartbeatName  = "heartbeat"
	coordinationHolderFileName = "holder"

	coordinationLockStaleAfter = 45 * time.Second
	coordinationRenewInterval  = 10 * time.Second
	coordinationRetryInterval  = 2 * time.Second
)

type Coordination struct {
	Mode     string
	HolderID string
}

type materializeRunner func(context.Context) (modelpackports.MaterializeResult, error)

type materializationLock struct {
	Path          string
	HeartbeatPath string
	HolderPath    string
	HolderID      string
}

func ValidateCoordination(cacheRoot string, cfg Coordination) error {
	if strings.TrimSpace(cfg.Mode) == "" {
		return nil
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return errors.New("materialization coordination requires cache-root mode")
	}
	switch {
	case strings.TrimSpace(cfg.Mode) != CoordinationModeShared:
		return errors.New("unsupported materialization coordination mode")
	case strings.TrimSpace(cfg.HolderID) == "":
		return errors.New("materialization coordination holder id must not be empty")
	default:
		return nil
	}
}

func MaterializeWithCoordination(
	ctx context.Context,
	cacheRoot string,
	destinationDir string,
	cfg Coordination,
	run materializeRunner,
) (modelpackports.MaterializeResult, error) {
	if strings.TrimSpace(cfg.Mode) == "" {
		return run(ctx)
	}
	if err := ValidateCoordination(cacheRoot, cfg); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	logger := slog.Default().With(
		slog.String("coordinationMode", cfg.Mode),
		slog.String("coordinationHolder", cfg.HolderID),
		slog.String("destinationDir", destinationDir),
	)

	lock := newMaterializationLock(cacheRoot, destinationDir, cfg.HolderID)
	waitLogged := false

	for {
		result, ready, err := readyMaterialization(cacheRoot, destinationDir)
		if err != nil {
			return modelpackports.MaterializeResult{}, err
		}
		if ready {
			logger.Info("coordinated materialization reused ready cache")
			return result, nil
		}

		acquired, release, err := tryAcquireMaterializationLock(lock)
		if err != nil {
			return modelpackports.MaterializeResult{}, err
		}
		if acquired {
			logger.Info("coordinated materialization lock acquired", slog.String("lockPath", lock.Path))
			result, ready, err = readyMaterialization(cacheRoot, destinationDir)
			if err != nil {
				release()
				return modelpackports.MaterializeResult{}, err
			}
			if ready {
				logger.Info("coordinated materialization reused ready cache after lock acquisition")
				release()
				return result, nil
			}

			result, err = run(ctx)
			if err != nil {
				release()
				return modelpackports.MaterializeResult{}, err
			}
			result, err = finalizeMaterialization(cacheRoot, destinationDir, result)
			release()
			return result, err
		}
		if !waitLogged {
			logger.Info("coordinated materialization waiting for active writer", slog.String("lockPath", lock.Path))
			waitLogged = true
		}

		select {
		case <-ctx.Done():
			return modelpackports.MaterializeResult{}, ctx.Err()
		case <-time.After(coordinationRetryInterval):
		}
	}
}

func readyMaterialization(cacheRoot, destinationDir string) (modelpackports.MaterializeResult, bool, error) {
	marker, err := ReadMarker(destinationDir)
	if err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	if marker == nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	modelPath := modelpackports.MaterializedModelPath(destinationDir)
	if _, err := os.Stat(modelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if err := UpdateCurrentLink(cacheRoot, modelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	if err := TouchUsage(destinationDir, time.Time{}); err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	return modelpackports.MaterializeResult{
		ModelPath:  CurrentLinkPath(cacheRoot),
		Digest:     strings.TrimSpace(marker.Digest),
		MediaType:  strings.TrimSpace(marker.MediaType),
		MarkerPath: MarkerPath(destinationDir),
	}, true, nil
}

func finalizeMaterialization(cacheRoot, destinationDir string, result modelpackports.MaterializeResult) (modelpackports.MaterializeResult, error) {
	if err := UpdateCurrentLink(cacheRoot, result.ModelPath); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	if err := TouchUsage(destinationDir, time.Time{}); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	slog.Default().Info(
		"coordinated materialization updated current cache link",
		slog.String("cacheRoot", cacheRoot),
		slog.String("destinationDir", destinationDir),
		slog.String("targetModelPath", result.ModelPath),
	)
	result.ModelPath = CurrentLinkPath(cacheRoot)
	return result, nil
}

func newMaterializationLock(cacheRoot, destinationDir, holderID string) materializationLock {
	digest := filepath.Base(filepath.Clean(destinationDir))
	lockPath := filepath.Join(filepath.Clean(cacheRoot), coordinationLockDirName, digest+".lock")
	return materializationLock{
		Path:          lockPath,
		HeartbeatPath: filepath.Join(lockPath, coordinationHeartbeatName),
		HolderPath:    filepath.Join(lockPath, coordinationHolderFileName),
		HolderID:      strings.TrimSpace(holderID),
	}
}

func tryAcquireMaterializationLock(lock materializationLock) (bool, func(), error) {
	if err := os.MkdirAll(filepath.Dir(lock.Path), 0o755); err != nil {
		return false, nil, err
	}
	if err := os.Mkdir(lock.Path, 0o755); err == nil {
		if err := writeMaterializationHeartbeat(lock); err != nil {
			_ = os.RemoveAll(lock.Path)
			return false, nil, err
		}
		return true, startMaterializationHeartbeat(lock), nil
	} else if !os.IsExist(err) {
		return false, nil, err
	}

	stale, err := materializationLockStale(lock)
	if err != nil {
		return false, nil, err
	}
	if stale {
		slog.Default().Warn("coordinated materialization removed stale lock", slog.String("lockPath", lock.Path))
		_ = os.RemoveAll(lock.Path)
	}
	return false, nil, nil
}

func materializationLockStale(lock materializationLock) (bool, error) {
	info, err := os.Stat(lock.HeartbeatPath)
	if errors.Is(err, os.ErrNotExist) {
		info, err = os.Stat(lock.Path)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return time.Since(info.ModTime()) > coordinationLockStaleAfter, nil
}

func startMaterializationHeartbeat(lock materializationLock) func() {
	stopCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(coordinationRenewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				_ = writeMaterializationHeartbeat(lock)
			}
		}
	}()
	return func() {
		close(stopCh)
		_ = os.RemoveAll(lock.Path)
	}
}

func writeMaterializationHeartbeat(lock materializationLock) error {
	if err := os.MkdirAll(lock.Path, 0o755); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := os.WriteFile(lock.HolderPath, []byte(lock.HolderID+"\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(lock.HeartbeatPath, []byte(now+"\n"), 0o644)
}
