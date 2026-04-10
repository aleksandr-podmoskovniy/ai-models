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
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/garbagecollection"

	"github.com/spf13/cobra"
)

func newGCCommand() *cobra.Command {
	options := garbagecollection.Options{
		RegistryBinary: garbagecollection.DefaultRegistryBinary,
		ConfigPath:     garbagecollection.DefaultConfigPath,
		GCTimeout:      10 * time.Minute,
		RescanInterval: garbagecollection.DefaultRescanInterval,
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
			client, err := garbagecollection.NewInClusterClient()
			if err != nil {
				return err
			}
			return garbagecollection.RunLoop(cmd.Context(), client, options)
		},
	}

	runCommand.Flags().StringVar(&options.RequestNamespace, "request-namespace", "", "Namespace containing DMCR garbage-collection request Secrets.")
	runCommand.Flags().StringVar(&options.RequestLabelSelector, "request-label-selector", garbagecollection.DefaultRequestLabelSelector(), "Label selector used to find DMCR garbage-collection request Secrets.")
	runCommand.Flags().StringVar(&options.RegistryBinary, "registry-binary", garbagecollection.DefaultRegistryBinary, "Path to the DMCR registry binary used for garbage-collect.")
	runCommand.Flags().StringVar(&options.ConfigPath, "config-path", garbagecollection.DefaultConfigPath, "Path to the active DMCR registry config file.")
	runCommand.Flags().DurationVar(&options.GCTimeout, "garbage-collection-timeout", 10*time.Minute, "Maximum time allowed for one registry garbage-collect run.")
	runCommand.Flags().DurationVar(&options.RescanInterval, "rescan-interval", garbagecollection.DefaultRescanInterval, "Polling interval used while waiting for new pending garbage-collection requests.")
	_ = runCommand.MarkFlagRequired("request-namespace")

	command.AddCommand(runCommand)
	return command
}
