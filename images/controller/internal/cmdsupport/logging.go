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

const DefaultLogFormat = "json"

func NewLogger(format string) (*slog.Logger, error) {
	return newLogger(format, os.Stderr)
}

func newLogger(format string, writer io.Writer) (*slog.Logger, error) {
	options := &slog.HandlerOptions{ReplaceAttr: normalizeLogAttr}

	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(writer, options)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(writer, options)), nil
	default:
		return nil, fmt.Errorf("unsupported log format %q", format)
	}
}

func NewComponentLogger(format, component string) (*slog.Logger, error) {
	logger, err := NewLogger(format)
	if err != nil {
		return nil, err
	}

	component = strings.TrimSpace(component)
	if component == "" {
		return logger, nil
	}
	return logger.With(slog.String("logger", component)), nil
}

func newComponentLogger(format, component string, writer io.Writer) (*slog.Logger, error) {
	logger, err := newLogger(format, writer)
	if err != nil {
		return nil, err
	}

	component = strings.TrimSpace(component)
	if component == "" {
		return logger, nil
	}
	return logger.With(slog.String("logger", component)), nil
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
