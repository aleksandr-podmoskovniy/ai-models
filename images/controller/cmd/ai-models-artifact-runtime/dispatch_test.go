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

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestConfigureRuntimeLoggerDefaultsToJSON(t *testing.T) {
	t.Setenv(logFormatEnv, "")

	previousSlog := slog.Default()
	previousControllerRuntime := logf.Log
	previousKlog := klog.Background()
	previousStderr := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	t.Cleanup(func() {
		slog.SetDefault(previousSlog)
		logf.SetLogger(previousControllerRuntime)
		klog.SetLogger(previousKlog)
		os.Stderr = previousStderr
		_ = readPipe.Close()
		_ = writePipe.Close()
	})
	os.Stderr = writePipe

	if err := configureRuntimeLogger("publish-worker"); err != nil {
		t.Fatalf("configureRuntimeLogger() error = %v", err)
	}

	slog.Default().Info("runtime default test", slog.String("runtimeKind", "materialize"))
	_ = writePipe.Close()

	var buffer bytes.Buffer
	if _, err := buffer.ReadFrom(readPipe); err != nil {
		t.Fatalf("ReadFrom() error = %v", err)
	}

	record := map[string]any{}
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got, want := record["logger"], "publish-worker"; got != want {
		t.Fatalf("logger = %#v, want %q", got, want)
	}
	if got, want := record["level"], "info"; got != want {
		t.Fatalf("level = %#v, want %q", got, want)
	}
	if got, want := record["runtime_kind"], "materialize"; got != want {
		t.Fatalf("runtime_kind = %#v, want %q", got, want)
	}
	if got, ok := record["ts"].(string); !ok || got == "" {
		t.Fatalf("ts = %#v, want non-empty string", record["ts"])
	}
}
