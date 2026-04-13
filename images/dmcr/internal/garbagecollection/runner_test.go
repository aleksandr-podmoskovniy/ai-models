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

	dmcrlogging "github.com/deckhouse/ai-models/dmcr/internal/logging"
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

func TestRunPendingRequestCycleMarksDoneAndLogs(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	logger, err := dmcrlogging.NewLogger("json")
	if err != nil {
		t.Fatalf("dmcrlogging.NewLogger() error = %v", err)
	}
	logger = slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{ReplaceAttr: replaceAttrForTest}))
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

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dmcr-gc-request-1",
			Namespace: "d8-ai-models",
			Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
			Annotations: map[string]string{
				switchAnnotationKey: "2026-04-13T00:00:00Z",
			},
		},
	}
	client := fake.NewSimpleClientset(secret.DeepCopy())
	options := Options{
		RequestNamespace:     "d8-ai-models",
		RequestLabelSelector: DefaultRequestLabelSelector(),
		RegistryBinary:       registryBinary,
		ConfigPath:           filepath.Join(t.TempDir(), "config.yml"),
		GCTimeout:            time.Minute,
	}
	finishedAt := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)

	handled, err := runPendingRequestCycle(context.Background(), client, options, func() time.Time { return finishedAt })
	if err != nil {
		t.Fatalf("runPendingRequestCycle() error = %v", err)
	}
	if !handled {
		t.Fatal("runPendingRequestCycle() = false, want true")
	}

	updated, err := client.CoreV1().Secrets("d8-ai-models").Get(context.Background(), "dmcr-gc-request-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get secret error = %v", err)
	}
	if got := updated.Annotations[doneAnnotationKey]; got != finishedAt.Format(time.RFC3339Nano) {
		t.Fatalf("done annotation = %q, want %q", got, finishedAt.Format(time.RFC3339Nano))
	}
	if _, exists := updated.Annotations[switchAnnotationKey]; exists {
		t.Fatal("expected switch annotation to be removed")
	}

	entries := decodeJSONLogLines(t, buffer.Bytes())
	assertLogMessage(t, entries, "dmcr garbage collection requested")
	assertLogMessage(t, entries, "dmcr garbage collection completed")
	assertLogMessage(t, entries, "dmcr garbage collection requests marked done")

	if got := entries[1]["registry_output"]; got != "gc-ok" {
		t.Fatalf("registry_output = %v, want gc-ok", got)
	}
}

func decodeJSONLogLines(t *testing.T, payload []byte) []map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(payload), []byte("\n"))
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", string(line), err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func assertLogMessage(t *testing.T, entries []map[string]any, message string) {
	t.Helper()

	for _, entry := range entries {
		if entry["msg"] == message {
			return
		}
	}
	t.Fatalf("log message %q not found in %#v", message, entries)
}

func replaceAttrForTest(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "ts"
	case slog.LevelKey:
		attr.Value = slog.StringValue("info")
	case slog.MessageKey:
		attr.Key = "msg"
	}
	return attr
}
