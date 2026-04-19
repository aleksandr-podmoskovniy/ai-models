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

import "time"

const (
	RuntimeCommand           = "node-cache-runtime"
	RuntimeCacheRootPath     = "/var/lib/ai-models/node-cache"
	RuntimeCacheRootEnv      = "AI_MODELS_NODE_CACHE_ROOT"
	RuntimeMaxSizeEnv        = "AI_MODELS_NODE_CACHE_MAX_TOTAL_SIZE"
	RuntimeMaxUnusedAgeEnv   = "AI_MODELS_NODE_CACHE_MAX_UNUSED_AGE"
	RuntimeScanIntervalEnv   = "AI_MODELS_NODE_CACHE_SCAN_INTERVAL"
	DefaultMaxUnusedAge      = 24 * time.Hour
	DefaultMaintenancePeriod = 5 * time.Minute
)
