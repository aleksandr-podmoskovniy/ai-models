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
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestShouldRunGarbageCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret corev1.Secret
		want   bool
	}{
		{
			name: "queued request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						RequestQueuedAtAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
		{
			name: "pending request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						switchAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: true,
		},
		{
			name: "done request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						doneAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
		{
			name: "non request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						switchAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldRunGarbageCollection(test.secret); got != test.want {
				t.Fatalf("shouldRunGarbageCollection() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestShouldActivateGarbageCollection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		secrets         []corev1.Secret
		activationDelay time.Duration
		want            bool
	}{
		{
			name: "queued request older than activation delay arms gc",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: now.Add(-11 * time.Minute).Format(time.RFC3339Nano),
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            true,
		},
		{
			name: "fresh queued request stays pending",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            false,
		},
		{
			name: "invalid queued timestamp arms gc fail-open",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: "broken",
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldActivateGarbageCollection(test.secrets, now, test.activationDelay); got != test.want {
				t.Fatalf("shouldActivateGarbageCollection() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRunRequestCycleArmsQueuedRequestsAndLogs(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: replaceAttrForTest}))
	logger = logger.With(slog.String("logger", "dmcr-garbage-collection"))

	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-13T13:40:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           filepath.Join(t.TempDir(), "config.yml"),
		ActivationDelay:      10 * time.Minute,
	}
	armedAt := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)

	handled, err := runRequestCycle(context.Background(), client, options, func() time.Time { return armedAt })
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	if !handled {
		t.Fatal("runRequestCycle() = false, want true")
	}

	updated, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-request-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret error = %v", err)
	}
	if got := updated.Annotations[switchAnnotationKey]; got != armedAt.Format(time.RFC3339Nano) {
		t.Fatalf("switch annotation = %q, want %q", got, armedAt.Format(time.RFC3339Nano))
	}
	if got := updated.Annotations[RequestQueuedAtAnnotationKey]; got == "" {
		t.Fatal("expected queued request timestamp to stay on armed secret")
	}

	entries := decodeJSONLogLines(t, buffer.Bytes())
	assertLogMessage(t, entries, "dmcr garbage collection maintenance cycle armed")
}

func TestRunLoopStepQueuesStartupBackfillRequest(t *testing.T) {
	client := fake.NewSimpleClientset()
	planner, err := newSchedulePlanner("0 2 * * *", time.Date(2026, 4, 23, 18, 25, 53, 0, time.UTC))
	if err != nil {
		t.Fatalf("newSchedulePlanner() error = %v", err)
	}

	previousRunner := startupBackfillCheckRunner
	startupBackfillCheckRunner = func(context.Context, string) (Report, error) {
		return Report{
			StaleDirectUploadMultipartUploads: []MultipartUploadInventoryEntry{
				{
					Prefix:    "dmcr/_ai_models/direct-upload/objects/session-a",
					ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-a/data",
					UploadID:  "upload-a",
					PartCount: 2,
				},
			},
		}, nil
	}
	t.Cleanup(func() {
		startupBackfillCheckRunner = previousRunner
	})

	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		ConfigPath:           filepath.Join(t.TempDir(), "config.yml"),
		ActivationDelay:      10 * time.Minute,
	}
	now := time.Date(2026, 4, 23, 18, 26, 0, 0, time.UTC)

	handled, err := runLoopStep(context.Background(), client, options, planner, func() time.Time { return now })
	if err != nil {
		t.Fatalf("runLoopStep() error = %v", err)
	}
	if handled {
		t.Fatal("runLoopStep() = true, want false while startup request is still queued")
	}

	secret, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), ScheduledRequestName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(secret) error = %v", err)
	}
	if got, want := secret.Annotations[RequestQueuedAtAnnotationKey], now.Format(time.RFC3339Nano); got != want {
		t.Fatalf("queued annotation = %q, want %q", got, want)
	}
}

func TestRunRequestCycleDeletesActiveRequestsAndLogs(t *testing.T) {
	var buffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: replaceAttrForTest}))
	logger = logger.With(slog.String("logger", "dmcr-garbage-collection"))

	previousLogger := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})

	registryBinary := filepath.Join(t.TempDir(), "dmcr")
	if err := os.WriteFile(registryBinary, []byte("#!/bin/sh\necho gc-ok\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte("storage:\n  maintenance:\n    readonly:\n      enabled: true\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}

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
		RegistryBinary:       registryBinary,
		ConfigPath:           configPath,
		GCTimeout:            time.Minute,
	}
	previousAutoCleanupRunner := autoCleanupRunner
	autoCleanupRunner = func(_ context.Context, configPath, registryBinary string, gcTimeout time.Duration, policy cleanupPolicy) (AutoCleanupResult, error) {
		_ = configPath
		_ = registryBinary
		_ = gcTimeout
		_ = policy
		return AutoCleanupResult{RegistryOutput: "gc-ok"}, nil
	}
	t.Cleanup(func() {
		autoCleanupRunner = previousAutoCleanupRunner
	})

	handled, err := runRequestCycle(context.Background(), client, options, func() time.Time { return time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	if !handled {
		t.Fatal("runRequestCycle() = false, want true")
	}

	if _, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-request-1", metav1.GetOptions{}); err == nil {
		t.Fatal("expected active request secret to be deleted after successful garbage collection")
	}

	entries := decodeJSONLogLines(t, buffer.Bytes())
	assertLogMessage(t, entries, "dmcr garbage collection requested")
	assertLogMessage(t, entries, "dmcr garbage collection completed")
	assertLogMessage(t, entries, "dmcr garbage collection requests removed")

	if got := entries[1]["registry_output"]; got != "gc-ok" {
		t.Fatalf("registry_output = %v, want gc-ok", got)
	}
}
