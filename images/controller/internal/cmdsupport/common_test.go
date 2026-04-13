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

package cmdsupport

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestSetDefaultLoggerBridgesControllerRuntimeAndKlog(t *testing.T) {
	var buffer bytes.Buffer

	previousSlog := slog.Default()
	previousControllerRuntime := logf.Log
	previousKlog := klog.Background()
	t.Cleanup(func() {
		slog.SetDefault(previousSlog)
		logf.SetLogger(previousControllerRuntime)
		klog.SetLogger(previousKlog)
	})

	logger, err := newLogger("json", &buffer)
	if err != nil {
		t.Fatalf("newLogger() error = %v", err)
	}
	SetDefaultLogger(logger)

	slog.Default().Info("slog message")
	logf.Log.WithName("controller-runtime").Info("controller-runtime message")
	klog.Background().WithName("klog").Info("klog message")

	rawOutput := buffer.Bytes()
	for _, line := range bytes.Split(bytes.TrimSpace(rawOutput), []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if got := bytes.Count(line, []byte(`"logger"`)); got > 1 {
			t.Fatalf("expected at most one logger field per line, got %d in %q", got, string(line))
		}
	}

	records := decodeLogRecords(t, rawOutput)
	for _, expected := range []string{
		"slog message",
		"controller-runtime message",
		"klog message",
	} {
		record := findRecordByMessage(t, records, expected)
		if got, ok := record["level"].(string); !ok || got == "" {
			t.Fatalf("expected record for %q to contain string level, got %#v", expected, record["level"])
		}
		if got, ok := record["ts"].(string); !ok || got == "" {
			t.Fatalf("expected record for %q to contain string ts, got %#v", expected, record["ts"])
		}
	}
}

func TestNewComponentLoggerAddsLoggerAttr(t *testing.T) {
	var buffer bytes.Buffer

	logger, err := newComponentLogger("json", "publish-worker", &buffer)
	if err != nil {
		t.Fatalf("newComponentLogger() error = %v", err)
	}
	logger.Info("component test message", slog.String("runtimeKind", "publish-worker"))

	record := findRecordByMessage(t, decodeLogRecords(t, buffer.Bytes()), "component test message")
	if got, want := record["logger"], "publish-worker"; got != want {
		t.Fatalf("logger attr = %#v, want %q", got, want)
	}
	if got, ok := record["level"].(string); !ok || got != "info" {
		t.Fatalf("level = %#v, want info", record["level"])
	}
	if got, want := record["runtime_kind"], "publish-worker"; got != want {
		t.Fatalf("runtime_kind attr = %#v, want %q", got, want)
	}
}

func TestCommandErrorUsesDefaultLogger(t *testing.T) {
	var buffer bytes.Buffer

	previousSlog := slog.Default()
	previousControllerRuntime := logf.Log
	previousKlog := klog.Background()
	t.Cleanup(func() {
		slog.SetDefault(previousSlog)
		logf.SetLogger(previousControllerRuntime)
		klog.SetLogger(previousKlog)
	})

	logger, err := newComponentLogger("json", "artifact-cleanup", &buffer)
	if err != nil {
		t.Fatalf("newComponentLogger() error = %v", err)
	}
	SetDefaultLogger(logger)

	if code := CommandError("artifact-cleanup", errors.New("boom")); code != 1 {
		t.Fatalf("unexpected exit code %d", code)
	}

	record := findRecordByMessage(t, decodeLogRecords(t, buffer.Bytes()), "artifact-cleanup exited with error")
	if got, want := record["logger"], "artifact-cleanup"; got != want {
		t.Fatalf("logger attr = %#v, want %q", got, want)
	}
	if got, want := record["level"], "error"; got != want {
		t.Fatalf("level = %#v, want %q", got, want)
	}
	if got, ok := record["error"].(string); !ok || !strings.Contains(got, "boom") {
		t.Fatalf("error field = %#v, want substring %q", record["error"], "boom")
	}
}

func decodeLogRecords(t *testing.T, output []byte) []map[string]any {
	t.Helper()

	lines := bytes.Split(bytes.TrimSpace(output), []byte("\n"))
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		record := map[string]any{}
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", string(line), err)
		}
		records = append(records, record)
	}

	return records
}

func findRecordByMessage(t *testing.T, records []map[string]any, message string) map[string]any {
	t.Helper()

	for _, record := range records {
		if got, _ := record["msg"].(string); got == message {
			return record
		}
	}

	t.Fatalf("did not find log record with msg %q in %#v", message, records)
	return nil
}
