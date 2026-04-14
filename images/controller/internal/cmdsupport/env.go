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
	"os"
	"strconv"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
	corev1 "k8s.io/api/core/v1"
)

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

	switch strings.ToLower(strings.TrimSpace(value)) {
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
