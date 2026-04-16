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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCleanupJobEnvKeepsParsedLogFormat(t *testing.T) {
	t.Setenv(logFormatEnv, "text")
	t.Setenv(logLevelEnv, "warn")
	t.Setenv("SSL_CERT_FILE", "/etc/custom/ca.pem")

	env := cleanupJobEnv("LOG_FORMAT,LOG_LEVEL,SSL_CERT_FILE", "json", "debug")

	if got := envValue(env, logFormatEnv); got != "json" {
		t.Fatalf("LOG_FORMAT = %q, want json", got)
	}
	if got := envValue(env, logLevelEnv); got != "debug" {
		t.Fatalf("LOG_LEVEL = %q, want debug", got)
	}
	if got := envValue(env, "SSL_CERT_FILE"); got != "/etc/custom/ca.pem" {
		t.Fatalf("SSL_CERT_FILE = %q, want /etc/custom/ca.pem", got)
	}
}

func envValue(env []corev1.EnvVar, name string) string {
	for _, item := range env {
		if item.Name == name {
			return item.Value
		}
	}
	return ""
}
