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
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRunRequestCycleBoundsFullActiveCleanupWindow(t *testing.T) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-13T13:40:00Z",
				switchAnnotationKey:          "2026-04-13T00:00:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           filepath.Join(t.TempDir(), "config.yml"),
		GCTimeout:            20 * time.Millisecond,
	}
	if err := os.WriteFile(options.ConfigPath, []byte("storage:\n  sealeds3: {}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}

	previousAutoCleanupRunner := autoCleanupRunner
	autoCleanupRunner = func(ctx context.Context, _ string, _ string, _ time.Duration, _ cleanupPolicy) (AutoCleanupResult, error) {
		<-ctx.Done()
		return AutoCleanupResult{}, ctx.Err()
	}
	t.Cleanup(func() {
		autoCleanupRunner = previousAutoCleanupRunner
	})

	startedAt := time.Now()
	handled, err := runRequestCycle(context.Background(), client, options, time.Now)
	if err == nil {
		t.Fatal("runRequestCycle() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runRequestCycle() error = %v, want context deadline exceeded", err)
	}
	if !handled {
		t.Fatal("runRequestCycle() = false, want true for attempted active cleanup")
	}
	if elapsed := time.Since(startedAt); elapsed > time.Second {
		t.Fatalf("active cleanup was not bounded by timeout, elapsed %s", elapsed)
	}
}
