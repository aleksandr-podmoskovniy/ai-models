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
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
	"github.com/distribution/distribution/v3/configuration"
	"github.com/distribution/distribution/v3/registry"
)

func registerMaintenanceGate() {
	checker, err := maintenance.NewFileCheckerFromEnv()
	if err != nil {
		fatalMaintenance("configure dmcr maintenance gate", err)
	}
	if checker == nil {
		return
	}
	registry.RegisterHandler(func(_ *configuration.Configuration, handler http.Handler) http.Handler {
		return maintenance.RegistryWriteGateHandler(checker, handler)
	})
	observer, err := maintenance.NewFileAckObserverFromEnv("dmcr", 0)
	if err != nil {
		fatalMaintenance("configure dmcr maintenance ack observer", err)
	}
	if observer != nil {
		observer.Start(context.Background())
	}
}

func fatalMaintenance(message string, err error) {
	slog.Default().Error(message, slog.Any("error", err))
	os.Exit(1)
}
