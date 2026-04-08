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

	logger := slog.New(slog.NewTextHandler(&buffer, nil))
	SetDefaultLogger(logger)

	slog.Default().Info("slog message")
	logf.Log.WithName("controller-runtime").Info("controller-runtime message")
	klog.Background().WithName("klog").Info("klog message")

	output := buffer.String()
	for _, expected := range []string{
		"slog message",
		"controller-runtime message",
		"klog message",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}
