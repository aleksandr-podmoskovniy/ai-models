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

package sourcefetch

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestHTTPAuthHeadersFromDirAuthorization(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(authDir, "authorization"), []byte("Bearer abc"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	headers, err := HTTPAuthHeadersFromDir(authDir)
	if err != nil {
		t.Fatalf("HTTPAuthHeadersFromDir() error = %v", err)
	}
	if got, want := headers["Authorization"], "Bearer abc"; got != want {
		t.Fatalf("unexpected authorization header %q", got)
	}
}

func TestHTTPAuthHeadersFromDirBasicPair(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(authDir, "username"), []byte("alice"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(authDir, "password"), []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	headers, err := HTTPAuthHeadersFromDir(authDir)
	if err != nil {
		t.Fatalf("HTTPAuthHeadersFromDir() error = %v", err)
	}
	if got, want := headers["Authorization"], "Basic YWxpY2U6c2VjcmV0"; got != want {
		t.Fatalf("unexpected authorization header %q", got)
	}
}

func TestFilenameFromHTTPResponse(t *testing.T) {
	t.Parallel()

	response := &http.Response{Header: http.Header{"Content-Disposition": []string{`attachment; filename="model.tar.gz"`}}}
	if got, want := filenameFromHTTPResponse("https://example.com/download", response), "model.tar.gz"; got != want {
		t.Fatalf("unexpected filename %q", got)
	}
}
