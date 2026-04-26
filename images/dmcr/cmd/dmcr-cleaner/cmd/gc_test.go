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

import "testing"

func TestGCCommandExposesOnlyGatedRunAndReadOnlyCheck(t *testing.T) {
	command := newGCCommand()
	commands := command.Commands()
	got := make(map[string]struct{}, len(commands))
	for _, subcommand := range commands {
		got[subcommand.Name()] = struct{}{}
	}

	if len(got) != 2 {
		t.Fatalf("gc subcommands = %v, want only run/check", got)
	}
	for _, name := range []string{"run", "check"} {
		if _, found := got[name]; !found {
			t.Fatalf("gc subcommands = %v, want %q", got, name)
		}
	}
}
