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

package uploadsession

import "testing"

func TestSanitizedUploadFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: "upload.bin"},
		{name: "basename", input: "model.tar.gz", want: "model.tar.gz"},
		{name: "path", input: "/tmp/model.gguf", want: "model.gguf"},
		{name: "windows path", input: `C:\tmp\model.gguf`, want: "model.gguf"},
		{name: "hidden", input: ".env", want: "upload.bin"},
		{name: "parent", input: "../evil.tar", want: "evil.tar"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizedUploadFileName(tc.input); got != tc.want {
				t.Fatalf("sanitizedUploadFileName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestNormalizePortDefaults(t *testing.T) {
	t.Parallel()

	if got, want := normalizePort(0), 8444; got != want {
		t.Fatalf("normalizePort(0) = %d, want %d", got, want)
	}
	if got, want := normalizePort(18080), 18080; got != want {
		t.Fatalf("normalizePort(18080) = %d, want %d", got, want)
	}
}

func TestRouteFromRequestPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		path        string
		wantSession string
		wantToken   string
		wantAction  string
		wantOK      bool
	}{
		{
			name:   "missing token",
			path:   "/v1/upload/session-a",
			wantOK: false,
		},
		{
			name:        "missing token action becomes token value",
			path:        "/v1/upload/session-a/probe",
			wantSession: "session-a",
			wantToken:   "probe",
			wantOK:      true,
		},
		{
			name:        "secret direct",
			path:        "/v1/upload/session-a/token-a",
			wantSession: "session-a",
			wantToken:   "token-a",
			wantOK:      true,
		},
		{
			name:        "secret action",
			path:        "/v1/upload/session-a/token-a/complete",
			wantSession: "session-a",
			wantToken:   "token-a",
			wantAction:  "/complete",
			wantOK:      true,
		},
		{
			name:   "unknown action",
			path:   "/v1/upload/session-a/token-a/unknown",
			wantOK: false,
		},
		{
			name:   "too deep",
			path:   "/v1/upload/session-a/token-a/probe/extra",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sessionID, token, action, ok := routeFromRequestPath(tc.path)
			if ok != tc.wantOK {
				t.Fatalf("routeFromRequestPath(%q) ok = %t, want %t", tc.path, ok, tc.wantOK)
			}
			if sessionID != tc.wantSession || token != tc.wantToken || action != tc.wantAction {
				t.Fatalf(
					"routeFromRequestPath(%q) = (%q, %q, %q), want (%q, %q, %q)",
					tc.path,
					sessionID,
					token,
					action,
					tc.wantSession,
					tc.wantToken,
					tc.wantAction,
				)
			}
		})
	}
}
