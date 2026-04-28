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

package logging

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestNewComponentLoggerUsesNormalizedJSONEnvelope(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	logger, err := newComponentLogger("json", "dmcr-garbage-collection", &buffer)
	if err != nil {
		t.Fatalf("newComponentLogger() error = %v", err)
	}

	logger.Info("gc started", "requestCount", 1)

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got := entry["level"]; got != "info" {
		t.Fatalf("level = %v, want info", got)
	}
	if _, ok := entry["ts"]; !ok {
		t.Fatal("expected ts field")
	}
	if got := entry["msg"]; got != "gc started" {
		t.Fatalf("msg = %v, want gc started", got)
	}
	if got := entry["logger"]; got != "dmcr-garbage-collection" {
		t.Fatalf("logger = %v, want dmcr-garbage-collection", got)
	}
	if got := entry["request_count"]; got != float64(1) {
		t.Fatalf("request_count = %v, want 1", got)
	}
}

func TestCommandErrorUsesNormalizedErrAttr(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := newComponentLogger("json", "dmcr-garbage-collection", &buffer)
	if err != nil {
		t.Fatalf("newComponentLogger() error = %v", err)
	}

	previous := slog.Default()
	SetDefaultLogger(logger)
	t.Cleanup(func() { SetDefaultLogger(previous) })

	if code := CommandError("dmcr-garbage-collection", errors.New("boom")); code != 1 {
		t.Fatalf("CommandError() code = %d, want 1", code)
	}

	var entry map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &entry); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if got, ok := entry["err"].(string); !ok || !strings.Contains(got, "boom") {
		t.Fatalf("err = %v, want boom substring", got)
	}
}
