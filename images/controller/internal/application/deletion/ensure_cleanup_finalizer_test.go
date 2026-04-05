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

package deletion

import (
	"errors"
	"testing"
)

func TestEnsureCleanupFinalizer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   EnsureCleanupFinalizerInput
		assert  func(t *testing.T, got EnsureCleanupFinalizerDecision)
		wantErr bool
	}{
		{
			name: "invalid handle fails closed",
			input: EnsureCleanupFinalizerInput{
				HandleErr: errors.New("broken"),
			},
			wantErr: true,
		},
		{
			name:  "missing handle without finalizer is noop",
			input: EnsureCleanupFinalizerInput{},
			assert: func(t *testing.T, got EnsureCleanupFinalizerDecision) {
				t.Helper()
				if got.AddFinalizer || got.RemoveFinalizer {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "missing handle removes stale finalizer",
			input: EnsureCleanupFinalizerInput{
				HasFinalizer: true,
			},
			assert: func(t *testing.T, got EnsureCleanupFinalizerDecision) {
				t.Helper()
				if !got.RemoveFinalizer || got.AddFinalizer {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "present handle adds finalizer",
			input: EnsureCleanupFinalizerInput{
				HandleFound: true,
			},
			assert: func(t *testing.T, got EnsureCleanupFinalizerDecision) {
				t.Helper()
				if !got.AddFinalizer || got.RemoveFinalizer {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
		{
			name: "present handle with finalizer is noop",
			input: EnsureCleanupFinalizerInput{
				HandleFound:  true,
				HasFinalizer: true,
			},
			assert: func(t *testing.T, got EnsureCleanupFinalizerDecision) {
				t.Helper()
				if got.AddFinalizer || got.RemoveFinalizer {
					t.Fatalf("unexpected decision %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := EnsureCleanupFinalizer(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("EnsureCleanupFinalizer() error = %v", err)
			}
			tc.assert(t, got)
		})
	}
}
