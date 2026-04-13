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
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type RepeatedStringFlag []string

const DefaultLogFormat = "json"

func (f *RepeatedStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *RepeatedStringFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*f = append(*f, value)
	return nil
}

func NewFlagSet(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	return set
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func RegistryAuthFromEnv(insecureEnv string) modelpackports.RegistryAuth {
	return modelpackports.RegistryAuth{
		Username: EnvOr("AI_MODELS_OCI_USERNAME", ""),
		Password: EnvOr("AI_MODELS_OCI_PASSWORD", ""),
		CAFile:   EnvOr("AI_MODELS_OCI_CA_FILE", ""),
		Insecure: EnvOrBool(insecureEnv, false),
	}
}

func EnvOr(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok && value != "" {
		return value
	}

	return fallback
}

func EnvOrBool(name string, fallback bool) bool {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func EnvOrInt(name string, fallback int) int {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func PassThroughEnv(csv string) []corev1.EnvVar {
	names := strings.Split(csv, ",")
	result := make([]corev1.EnvVar, 0, len(names))
	seen := map[string]struct{}{}

	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, corev1.EnvVar{Name: name, Value: value})
	}

	return result
}

func FallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}

	return fallback
}

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

func WriteTerminationFailure(message string) {
	WriteTerminationMessage(strings.TrimSpace(message))
}

func WriteTerminationMessage(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	_ = os.WriteFile("/dev/termination-log", []byte(message), 0o644)
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
