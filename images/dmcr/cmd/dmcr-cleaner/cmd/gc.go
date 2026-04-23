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
	"log/slog"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/garbagecollection"

	"github.com/spf13/cobra"
)

func newGCCommand() *cobra.Command {
	options := garbagecollection.Options{
		RegistryBinary:  garbagecollection.DefaultRegistryBinary,
		ConfigPath:      garbagecollection.DefaultConfigPath,
		GCTimeout:       10 * time.Minute,
		RescanInterval:  garbagecollection.DefaultRescanInterval,
		ActivationDelay: garbagecollection.DefaultActivationDelay,
	}

	command := &cobra.Command{
		Use:   "gc",
		Short: "Run DMCR garbage collection lifecycle operations",
	}
	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Run registry garbage-collect while DMCR stays in maintenance mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			slog.Default().Info(
				"dmcr garbage collection helper started",
				slog.String("request_namespace", options.RequestNamespace),
				slog.String("request_label_selector", options.RequestLabelSelector),
				slog.Duration("garbage_collection_timeout", options.GCTimeout),
				slog.Duration("rescan_interval", options.RescanInterval),
				slog.Duration("activation_delay", options.ActivationDelay),
				slog.String("schedule", options.Schedule),
			)
			client, err := garbagecollection.NewInClusterClient()
			if err != nil {
				return err
			}
			if err := garbagecollection.RunLoop(cmd.Context(), client, options); err != nil {
				return err
			}
			slog.Default().Info("dmcr garbage collection helper stopped")
			return nil
		},
	}
	checkCommand := &cobra.Command{
		Use:   "check",
		Short: "Report stale DMCR repository, source-mirror, direct-upload object, and direct-upload multipart residue that no longer belong to live Model or ClusterModel objects",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report, err := garbagecollection.Check(cmd.Context(), options.ConfigPath)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write([]byte(report.Format()))
			return err
		},
	}
	autoCleanupCommand := &cobra.Command{
		Use:   "auto-cleanup",
		Short: "Delete stale DMCR repository, source-mirror, and direct-upload residue, then run registry garbage-collect",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := garbagecollection.AutoCleanup(cmd.Context(), options.ConfigPath, options.RegistryBinary, options.GCTimeout)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write([]byte(result.Report.Format()))
			if err != nil {
				return err
			}
			if result.RegistryOutput != "" {
				_, err = cmd.OutOrStdout().Write([]byte(result.RegistryOutput + "\n"))
			}
			return err
		},
	}

	runCommand.Flags().StringVar(&options.RequestNamespace, "request-namespace", "", "Namespace containing DMCR garbage-collection request Secrets.")
	runCommand.Flags().StringVar(&options.RequestLabelSelector, "request-label-selector", garbagecollection.DefaultRequestLabelSelector(), "Label selector used to find DMCR garbage-collection request Secrets.")
	runCommand.Flags().StringVar(&options.RegistryBinary, "registry-binary", garbagecollection.DefaultRegistryBinary, "Path to the DMCR registry binary used for garbage-collect.")
	runCommand.Flags().StringVar(&options.ConfigPath, "config-path", garbagecollection.DefaultConfigPath, "Path to the active DMCR registry config file.")
	runCommand.Flags().DurationVar(&options.GCTimeout, "garbage-collection-timeout", 10*time.Minute, "Maximum time allowed for one registry garbage-collect run.")
	runCommand.Flags().DurationVar(&options.RescanInterval, "rescan-interval", garbagecollection.DefaultRescanInterval, "Polling interval used while waiting for new pending garbage-collection requests.")
	runCommand.Flags().DurationVar(&options.ActivationDelay, "activation-delay", garbagecollection.DefaultActivationDelay, "Minimum time a queued request must stay pending before the helper arms a maintenance GC cycle.")
	runCommand.Flags().StringVar(&options.Schedule, "schedule", "", "Cron schedule used to enqueue periodic stale-sweep requests; empty disables the periodic trigger.")
	_ = runCommand.MarkFlagRequired("request-namespace")
	checkCommand.Flags().StringVar(&options.ConfigPath, "config-path", garbagecollection.DefaultConfigPath, "Path to the active DMCR registry config file.")
	autoCleanupCommand.Flags().StringVar(&options.RegistryBinary, "registry-binary", garbagecollection.DefaultRegistryBinary, "Path to the DMCR registry binary used for garbage-collect.")
	autoCleanupCommand.Flags().StringVar(&options.ConfigPath, "config-path", garbagecollection.DefaultConfigPath, "Path to the active DMCR registry config file.")
	autoCleanupCommand.Flags().DurationVar(&options.GCTimeout, "garbage-collection-timeout", 10*time.Minute, "Maximum time allowed for one registry garbage-collect run.")

	command.AddCommand(runCommand)
	command.AddCommand(checkCommand)
	command.AddCommand(autoCleanupCommand)
	return command
}
