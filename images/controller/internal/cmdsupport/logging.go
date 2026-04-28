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
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"unicode"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	DefaultLogFormat = "json"
	DefaultLogLevel  = "info"
)

func NewLogger(format, level string) (*slog.Logger, error) {
	return newLogger(format, level, os.Stderr)
}

func newLogger(format, level string, writer io.Writer) (*slog.Logger, error) {
	resolvedLevel, err := parseLogLevel(level)
	if err != nil {
		return nil, err
	}
	options := &slog.HandlerOptions{
		Level:       resolvedLevel,
		ReplaceAttr: normalizeLogAttr,
	}

	switch format {
	case "text":
		return slog.New(newDedupeHandler(slog.NewTextHandler(writer, options))), nil
	case "json":
		return slog.New(newDedupeHandler(slog.NewJSONHandler(writer, options))), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func NewComponentLogger(format, level, component string) (*slog.Logger, error) {
	logger, err := NewLogger(format, level)
	if err != nil {
		return nil, err
	}

	component = strings.TrimSpace(component)
	if component == "" {
		return logger, nil
	}
	return logger.With(slog.String("component", component)), nil
}

func newComponentLogger(format, level, component string, writer io.Writer) (*slog.Logger, error) {
	logger, err := newLogger(format, level, writer)
	if err != nil {
		return nil, err
	}

	component = strings.TrimSpace(component)
	if component == "" {
		return logger, nil
	}
	return logger.With(slog.String("component", component)), nil
}

func SetDefaultLogger(logger *slog.Logger) {
	slog.SetDefault(logger)
	bridged := logr.FromSlogHandler(logger.Handler())
	logf.SetLogger(bridged)
	klog.SetLogger(bridged)
}

func CommandError(name string, err error) int {
	slog.Default().Error(name+" exited with error", slog.Any("error", err))
	return 1
}

func normalizeLogAttr(_ []string, attr slog.Attr) slog.Attr {
	switch attr.Key {
	case slog.TimeKey:
		attr.Key = "ts"
	case slog.LevelKey:
		attr.Value = slog.StringValue(strings.ToLower(attr.Value.String()))
	case slog.MessageKey:
		attr.Key = "msg"
	default:
		attr.Key = normalizeLogKey(attr.Key)
	}

	return attr
}

func normalizeLogKey(key string) string {
	if key == "" {
		return key
	}
	if key == "error" {
		return "err"
	}

	var builder strings.Builder
	runes := []rune(key)
	for index, current := range runes {
		if unicode.IsUpper(current) {
			if index > 0 {
				previous := runes[index-1]
				nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
				if unicode.IsLower(previous) || unicode.IsDigit(previous) || nextIsLower {
					builder.WriteByte('_')
				}
			}
			builder.WriteRune(unicode.ToLower(current))
			continue
		}
		builder.WriteRune(current)
	}

	return builder.String()
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", DefaultLogLevel:
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", level)
	}
}

type dedupeHandler struct {
	next        slog.Handler
	contextKeys map[string]struct{}
	groups      []string
}

func newDedupeHandler(next slog.Handler) slog.Handler {
	return &dedupeHandler{
		next:        next,
		contextKeys: make(map[string]struct{}),
	}
}

func (h *dedupeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *dedupeHandler) Handle(ctx context.Context, record slog.Record) error {
	seen := copyStringSet(h.contextKeys)
	filtered := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		key := h.attrKey(attr)
		if _, exists := seen[key]; exists {
			return true
		}
		seen[key] = struct{}{}
		filtered.AddAttrs(attr)
		return true
	})
	return h.next.Handle(ctx, filtered)
}

func (h *dedupeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	keys := copyStringSet(h.contextKeys)
	filtered := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		key := h.attrKey(attr)
		if _, exists := keys[key]; exists {
			continue
		}
		keys[key] = struct{}{}
		filtered = append(filtered, attr)
	}
	return &dedupeHandler{
		next:        h.next.WithAttrs(filtered),
		contextKeys: keys,
		groups:      append([]string(nil), h.groups...),
	}
}

func (h *dedupeHandler) WithGroup(name string) slog.Handler {
	groups := append([]string(nil), h.groups...)
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		groups = append(groups, trimmed)
	}
	return &dedupeHandler{
		next:        h.next.WithGroup(name),
		contextKeys: copyStringSet(h.contextKeys),
		groups:      groups,
	}
}

func (h *dedupeHandler) attrKey(attr slog.Attr) string {
	key := normalizeLogKey(attr.Key)
	if len(h.groups) == 0 {
		return key
	}
	return strings.Join(append(append([]string(nil), h.groups...), key), ".")
}

func copyStringSet(input map[string]struct{}) map[string]struct{} {
	output := make(map[string]struct{}, len(input))
	for key := range input {
		output[key] = struct{}{}
	}
	return output
}
