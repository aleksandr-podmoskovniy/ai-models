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
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const (
	materializeCoordinationModeEnv   = "AI_MODELS_MATERIALIZE_COORDINATION_MODE"
	materializeCoordinationHolderEnv = "AI_MODELS_MATERIALIZE_COORDINATION_HOLDER_ID"

	materializeCoordinationModeShared = "shared-cache"

	coordinationLockDirName    = ".coordination"
	coordinationHeartbeatName  = "heartbeat"
	coordinationHolderFileName = "holder"

	coordinationLockStaleAfter = 45 * time.Second
	coordinationRenewInterval  = 10 * time.Second
	coordinationRetryInterval  = 2 * time.Second
)

type materializeRunner func(context.Context) (modelpackports.MaterializeResult, error)

type materializeCoordination struct {
	Mode     string
	HolderID string
}

type materializedMarkerSnapshot struct {
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType,omitempty"`
}

type materializationLock struct {
	Path          string
	HeartbeatPath string
	HolderPath    string
	HolderID      string
}

func resolveMaterializeCoordination(cacheRoot string) (materializeCoordination, error) {
	mode := strings.TrimSpace(os.Getenv(materializeCoordinationModeEnv))
	if mode == "" {
		return materializeCoordination{}, nil
	}
	if strings.TrimSpace(cacheRoot) == "" {
		return materializeCoordination{}, errors.New("materialization coordination requires cache-root mode")
	}

	holderID := strings.TrimSpace(os.Getenv(materializeCoordinationHolderEnv))
	if holderID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return materializeCoordination{}, err
		}
		holderID = strings.TrimSpace(hostname)
	}

	switch {
	case mode != materializeCoordinationModeShared:
		return materializeCoordination{}, errors.New("unsupported materialization coordination mode")
	case holderID == "":
		return materializeCoordination{}, errors.New("materialization coordination holder id must not be empty")
	}

	return materializeCoordination{
		Mode:     mode,
		HolderID: holderID,
	}, nil
}

func materializeWithCoordination(
	ctx context.Context,
	cacheRoot string,
	destinationDir string,
	cfg materializeCoordination,
	run materializeRunner,
) (modelpackports.MaterializeResult, error) {
	if cfg.Mode == "" {
		return run(ctx)
	}

	lock := newMaterializationLock(cacheRoot, destinationDir, cfg.HolderID)

	for {
		result, ready, err := readyCoordinatedMaterialization(cacheRoot, destinationDir)
		if err != nil {
			return modelpackports.MaterializeResult{}, err
		}
		if ready {
			return result, nil
		}

		acquired, release, err := tryAcquireMaterializationLock(lock)
		if err != nil {
			return modelpackports.MaterializeResult{}, err
		}
		if acquired {
			result, ready, err = readyCoordinatedMaterialization(cacheRoot, destinationDir)
			if err != nil {
				release()
				return modelpackports.MaterializeResult{}, err
			}
			if ready {
				release()
				return result, nil
			}

			result, err = run(ctx)
			if err != nil {
				release()
				return modelpackports.MaterializeResult{}, err
			}
			result, err = finalizeCoordinatedMaterialization(cacheRoot, destinationDir, result)
			release()
			return result, err
		}

		select {
		case <-ctx.Done():
			return modelpackports.MaterializeResult{}, ctx.Err()
		case <-time.After(coordinationRetryInterval):
		}
	}
}

func readyCoordinatedMaterialization(cacheRoot, destinationDir string) (modelpackports.MaterializeResult, bool, error) {
	markerPath := filepath.Join(destinationDir, ".ai-models-materialized.json")
	body, err := os.ReadFile(markerPath)
	if errors.Is(err, os.ErrNotExist) {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	modelPath := modelpackports.MaterializedModelPath(destinationDir)
	if _, err := os.Stat(modelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, nil
	}
	if err := updateCurrentMaterializationLink(cacheRoot, modelPath); err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	var marker materializedMarkerSnapshot
	if err := json.Unmarshal(body, &marker); err != nil {
		return modelpackports.MaterializeResult{}, false, err
	}
	return modelpackports.MaterializeResult{
		ModelPath:  filepath.Join(filepath.Clean(cacheRoot), cacheCurrentPath),
		Digest:     strings.TrimSpace(marker.Digest),
		MediaType:  strings.TrimSpace(marker.MediaType),
		MarkerPath: markerPath,
	}, true, nil
}

func finalizeCoordinatedMaterialization(cacheRoot, destinationDir string, result modelpackports.MaterializeResult) (modelpackports.MaterializeResult, error) {
	if err := updateCurrentMaterializationLink(cacheRoot, result.ModelPath); err != nil {
		return modelpackports.MaterializeResult{}, err
	}
	result.ModelPath = filepath.Join(filepath.Clean(cacheRoot), cacheCurrentPath)
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
