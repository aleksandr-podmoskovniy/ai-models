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
	"os"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

const (
	materializeCoordinationModeEnv   = "AI_MODELS_MATERIALIZE_COORDINATION_MODE"
	materializeCoordinationHolderEnv = "AI_MODELS_MATERIALIZE_COORDINATION_HOLDER_ID"
)

type materializeCoordination = nodecache.Coordination

func resolveMaterializeCoordination(cacheRoot string) (materializeCoordination, error) {
	mode := strings.TrimSpace(os.Getenv(materializeCoordinationModeEnv))
	if mode == "" {
		return materializeCoordination{}, nil
	}

	holderID := strings.TrimSpace(os.Getenv(materializeCoordinationHolderEnv))
	if holderID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return materializeCoordination{}, err
		}
		holderID = strings.TrimSpace(hostname)
	}

	cfg := materializeCoordination{
		Mode:     mode,
		HolderID: holderID,
	}
	if err := nodecache.ValidateCoordination(cacheRoot, cfg); err != nil {
		return materializeCoordination{}, err
	}
	return cfg, nil
}
