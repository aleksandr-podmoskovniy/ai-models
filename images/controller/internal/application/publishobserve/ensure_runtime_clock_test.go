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

package publishobserve

import (
	"testing"
	"time"
)

func TestRuntimeObservationNowDefaultsToCurrentUTC(t *testing.T) {
	t.Parallel()

	if got := runtimeObservationNow(time.Time{}); got.IsZero() {
		t.Fatal("expected non-zero current time")
	}
	if got := runtimeObservationNow(time.Date(2026, 4, 7, 12, 0, 0, 0, time.FixedZone("custom", 3*60*60))); got.Location() != time.UTC {
		t.Fatalf("expected UTC location, got %v", got.Location())
	}
}
