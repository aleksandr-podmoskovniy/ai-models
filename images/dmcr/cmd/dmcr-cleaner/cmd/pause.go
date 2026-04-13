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

package cmd

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newPauseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pause",
		Short: "pause waits for Pod termination while DMCR is in normal mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			slog.Default().Info("dmcr garbage collection helper started in pause mode")
			waitForTermination(cmd.Context())
			slog.Default().Info("dmcr garbage collection helper stopped")
			return nil
		},
	}
}

func waitForTermination(ctx context.Context) {
	signalContext, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-signalContext.Done()
}
