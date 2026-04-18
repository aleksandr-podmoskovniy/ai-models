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
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *writableRegistryState) handleManifest(w http.ResponseWriter, r *http.Request, ref string) {
	switch r.Method {
	case http.MethodPut:
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			s.t.Fatalf("ReadAll(manifest body) error = %v", err)
		}
		digestBytes := sha256.Sum256(payload)
		digest := "sha256:" + hex.EncodeToString(digestBytes[:])
		s.manifests[digest] = payload
		if !strings.HasPrefix(ref, "sha256:") {
			s.tags[ref] = digest
		}
		w.Header().Set("Docker-Content-Digest", digest)
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		digest := ref
		if !strings.HasPrefix(digest, "sha256:") {
			digest = s.tags[ref]
		}
		payload, ok := s.manifests[digest]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Content-Type", ManifestMediaType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	case http.MethodDelete:
		delete(s.manifests, ref)
		for tag, digest := range s.tags {
			if digest == ref {
				delete(s.tags, tag)
			}
		}
		w.WriteHeader(http.StatusAccepted)
	default:
		s.t.Fatalf("unexpected manifest method %s", r.Method)
	}
}

func (s *writableRegistryState) handleBlob(w http.ResponseWriter, r *http.Request, digest string) {
	payload, ok := s.blobs[digest]
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Docker-Content-Digest", digest)
	w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
