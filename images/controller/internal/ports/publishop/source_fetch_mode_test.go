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

package publishop

import "testing"

func TestNormalizeSourceFetchModeDefaultsToDirect(t *testing.T) {
	t.Parallel()

	if got, want := NormalizeSourceFetchMode(""), SourceFetchModeDirect; got != want {
		t.Fatalf("NormalizeSourceFetchMode(\"\") = %q, want %q", got, want)
	}
}

func TestValidateSourceFetchMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   SourceFetchMode
		wantErr bool
	}{
		{name: "mirror", input: SourceFetchModeMirror},
		{name: "direct", input: SourceFetchModeDirect},
		{name: "mixed case direct", input: "Direct"},
		{name: "unsupported", input: "broken", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSourceFetchMode(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateSourceFetchMode() error = %v", err)
			}
		})
	}
}
