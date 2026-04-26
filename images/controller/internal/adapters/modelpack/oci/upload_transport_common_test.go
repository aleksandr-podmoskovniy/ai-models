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

package oci

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWaitDirectUploadRecoveryRetryIsBoundedAndContextAware(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	recoveries := 0
	retryWait := time.Millisecond
	cause := errors.New("temporary object store failure")
	err := waitDirectUploadRecoveryRetry(ctx, &recoveries, &retryWait, cause)
	if err == nil {
		t.Fatal("waitDirectUploadRecoveryRetry() error = nil, want context cancellation")
	}
	if !strings.Contains(err.Error(), cause.Error()) {
		t.Fatalf("error = %v, want original cause", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if recoveries != 1 {
		t.Fatalf("recoveries = %d, want 1", recoveries)
	}
}

func TestWaitDirectUploadRecoveryRetryStopsAfterBudget(t *testing.T) {
	t.Parallel()

	recoveries := blobUploadRecoveryAttempts
	retryWait := time.Millisecond
	cause := errors.New("temporary object store failure")
	err := waitDirectUploadRecoveryRetry(context.Background(), &recoveries, &retryWait, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("waitDirectUploadRecoveryRetry() error = %v, want %v", err, cause)
	}
	if recoveries != blobUploadRecoveryAttempts+1 {
		t.Fatalf("recoveries = %d, want %d", recoveries, blobUploadRecoveryAttempts+1)
	}
}
