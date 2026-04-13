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
	"fmt"
	"log/slog"
	"os"

	"github.com/deckhouse/ai-models/dmcr/cmd/dmcr-cleaner/cmd"
	"github.com/deckhouse/ai-models/dmcr/internal/logging"
)

const logFormatEnv = "LOG_FORMAT"

func main() {
	logger, err := logging.NewComponentLogger(logging.EnvOr(logFormatEnv, logging.DefaultLogFormat), "dmcr-garbage-collection")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	logging.SetDefaultLogger(logger)

	if err := cmd.NewRootCommand().Execute(); err != nil {
		slog.Default().Error("dmcr-cleaner exited with error", slog.Any("error", err))
		os.Exit(1)
	}
}
