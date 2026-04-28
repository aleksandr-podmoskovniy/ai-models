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

package storageusage

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestCollectorReportsUnknownCapacityWhenStoreDisabled(t *testing.T) {
	registry := prometheus.NewRegistry()
	SetupCollector(nil, registry, nil)

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if len(families) != 5 {
		t.Fatalf("metric family count = %d, want 5", len(families))
	}
}
