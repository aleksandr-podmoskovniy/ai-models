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

	"github.com/distribution/distribution/v3/registry"
)

func TestConfigureCommandsBrandsRegistryAsDMCR(t *testing.T) {
	t.Parallel()

	originalRootUse := registry.RootCmd.Use
	originalRootShort := registry.RootCmd.Short
	originalRootLong := registry.RootCmd.Long
	originalServeUse := registry.ServeCmd.Use
	originalServeShort := registry.ServeCmd.Short
	originalServeLong := registry.ServeCmd.Long
	t.Cleanup(func() {
		registry.RootCmd.Use = originalRootUse
		registry.RootCmd.Short = originalRootShort
		registry.RootCmd.Long = originalRootLong
		registry.ServeCmd.Use = originalServeUse
		registry.ServeCmd.Short = originalServeShort
		registry.ServeCmd.Long = originalServeLong
	})

	configureCommands()

	if got, want := registry.RootCmd.Use, "dmcr"; got != want {
		t.Fatalf("RootCmd.Use = %q, want %q", got, want)
	}
	if got, want := registry.ServeCmd.Use, "serve <config>"; got != want {
		t.Fatalf("ServeCmd.Use = %q, want %q", got, want)
	}
}
