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

package garbagecollection

import (
	"strings"
	"testing"
)

func TestBoundedCommandOutputMessageSummarizesLargeOutput(t *testing.T) {
	t.Parallel()

	message := boundedCommandOutputMessage(strings.Join([]string{
		"line-1",
		"line-2",
		"line-3",
		"line-4",
		"line-5",
		"line-6",
		"line-7",
		"line-8",
		"line-9",
	}, "\n"), "fallback")

	for _, expected := range []string{
		"output_line_count=9",
		"output_sha256=",
		`first_lines="line-1\nline-2"`,
		`last_lines="line-8\nline-9"`,
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("summary %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "line-5") {
		t.Fatalf("summary must not include full output: %q", message)
	}
}
