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

package oci

import (
	"net/http"
	"strings"
	"testing"
)

type writableRegistryState struct {
	t                   *testing.T
	uploads             map[string][]byte
	blobs               map[string][]byte
	manifests           map[string][]byte
	tags                map[string]string
	patches             int
	statuses            int
	interruptFirstPatch bool
	interrupted         bool
}

func newWritableRegistryState(t *testing.T, interruptFirstPatch bool) *writableRegistryState {
	return &writableRegistryState{
		t:                   t,
		uploads:             make(map[string][]byte),
		blobs:               make(map[string][]byte),
		manifests:           make(map[string][]byte),
		tags:                make(map[string]string),
		interruptFirstPatch: interruptFirstPatch,
	}
}

func (s *writableRegistryState) serve(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	if !ok || user != "writer" || pass != "secret" {
		s.t.Fatalf("unexpected auth %q/%q", user, pass)
	}

	const repoPrefix = "/v2/ai-models/catalog/model"
	switch {
	case r.Method == http.MethodPost && r.URL.Path == repoPrefix+"/blobs/uploads/":
		s.handleStartUpload(w)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/uploads/"):
		s.handleUploadStatus(w, r)
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/uploads/"):
		s.handleUploadPatch(w, r)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/uploads/"):
		s.handleUploadFinalize(w, r)
	case strings.HasPrefix(r.URL.Path, repoPrefix+"/manifests/"):
		s.handleManifest(w, r, strings.TrimPrefix(r.URL.Path, repoPrefix+"/manifests/"))
	case (r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.HasPrefix(r.URL.Path, repoPrefix+"/blobs/"):
		s.handleBlob(w, r, strings.TrimPrefix(r.URL.Path, repoPrefix+"/blobs/"))
	default:
		s.t.Fatalf("unexpected path %s %q", r.Method, r.URL.Path)
	}
}
