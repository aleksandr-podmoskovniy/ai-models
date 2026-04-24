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
	"encoding/json"
	"log/slog"
	"testing"
)

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
