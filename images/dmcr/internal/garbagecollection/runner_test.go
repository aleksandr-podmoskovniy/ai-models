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
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

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
	if got, want := updated.Annotations[phaseAnnotationKey], phaseArmed; got != want {
		t.Fatalf("phase annotation = %q, want %q", got, want)
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

func TestRunRequestCycleMarksActiveRequestsDoneAndLogs(t *testing.T) {
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
	if err := os.WriteFile(configPath, []byte("storage:\n  sealeds3: {}\n"), 0o644); err != nil {
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
		Data: map[string][]byte{
			directUploadTokenDataKey: []byte("token-must-not-survive-result"),
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
	previousCleanupRunner := cleanupRunner
	cleanupRunner = func(_ context.Context, configPath, registryBinary string, gcTimeout time.Duration, policy cleanupPolicy) (CleanupResult, error) {
		_ = configPath
		_ = registryBinary
		_ = gcTimeout
		_ = policy
		return CleanupResult{RegistryOutput: "gc-ok"}, nil
	}
	t.Cleanup(func() {
		cleanupRunner = previousCleanupRunner
	})

	handled, err := runRequestCycle(context.Background(), client, options, func() time.Time { return time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("runRequestCycle() error = %v", err)
	}
	if !handled {
		t.Fatal("runRequestCycle() = false, want true")
	}

	updated, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-request-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected completed request secret to stay observable: %v", err)
	}
	if got, want := updated.Annotations[phaseAnnotationKey], phaseDone; got != want {
		t.Fatalf("phase annotation = %q, want %q", got, want)
	}
	if updated.Annotations[switchAnnotationKey] != "" {
		t.Fatalf("expected completed request switch annotation to be removed, got %#v", updated.Annotations)
	}
	if updated.Annotations[completedAtAnnotationKey] == "" {
		t.Fatalf("expected completed-at annotation, got %#v", updated.Annotations)
	}
	if _, found := updated.Data[directUploadTokenDataKey]; found {
		t.Fatalf("completed result must not retain direct-upload token data: %#v", updated.Data)
	}
	var result requestResultRecord
	if err := json.Unmarshal(updated.Data[resultDataKey], &result); err != nil {
		t.Fatalf("decode result data error = %v", err)
	}
	if got, want := result.RegistryOutput, "gc-ok"; got != want {
		t.Fatalf("result registry output = %q, want %q", got, want)
	}

	entries := decodeJSONLogLines(t, buffer.Bytes())
	assertLogMessage(t, entries, "dmcr garbage collection requested")
	assertLogMessage(t, entries, "dmcr garbage collection completed")
	assertLogMessage(t, entries, "dmcr garbage collection requests completed")

	if got := entries[1]["registry_output"]; got != "gc-ok" {
		t.Fatalf("registry_output = %v, want gc-ok", got)
	}
}
