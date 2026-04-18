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
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *writableRegistryState) handleStartUpload(w http.ResponseWriter) {
	uploadID := "upload-" + hex.EncodeToString([]byte{byte(len(s.uploads) + 1)})
	s.uploads[uploadID] = nil
	w.Header().Set("Location", "/uploads/"+uploadID)
	w.WriteHeader(http.StatusAccepted)
}

func (s *writableRegistryState) handleUploadStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
	payload, ok := s.uploads[uploadID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.statuses++
	w.Header().Set("Location", "/uploads/"+uploadID)
	if len(payload) > 0 {
		w.Header().Set("Range", "0-"+strconv.Itoa(len(payload)-1))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *writableRegistryState) handleUploadPatch(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
	current, ok := s.uploads[uploadID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	rangeHeader := strings.TrimSpace(r.Header.Get("Content-Range"))
	parts := strings.Split(rangeHeader, "-")
	if len(parts) != 2 {
		s.t.Fatalf("unexpected Content-Range %q", rangeHeader)
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		s.t.Fatalf("Atoi(start) error = %v", err)
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		s.t.Fatalf("Atoi(end) error = %v", err)
	}
	if start != len(current) {
		if len(current) > 0 {
			w.Header().Set("Range", "0-"+strconv.Itoa(len(current)-1))
		}
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Fatalf("ReadAll(patch body) error = %v", err)
	}
	if got, want := len(payload), end-start+1; got != want {
		s.t.Fatalf("patch body length = %d, want %d", got, want)
	}
	s.uploads[uploadID] = append(current, payload...)
	s.patches++
	if s.interruptFirstPatch && !s.interrupted {
		s.interrupted = true
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			s.t.Fatal("response writer does not support hijacking")
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			s.t.Fatalf("Hijack() error = %v", err)
		}
		_ = conn.Close()
		return
	}
	w.Header().Set("Location", "/uploads/"+uploadID)
	w.Header().Set("Range", "0-"+strconv.Itoa(len(s.uploads[uploadID])-1))
	w.WriteHeader(http.StatusAccepted)
}

func (s *writableRegistryState) handleUploadFinalize(w http.ResponseWriter, r *http.Request) {
	uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
	digest := strings.TrimSpace(r.URL.Query().Get("digest"))
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Fatalf("ReadAll(upload body) error = %v", err)
	}
	if uploadID == "" || digest == "" {
		s.t.Fatalf("unexpected upload request %q?%q", r.URL.Path, r.URL.RawQuery)
	}
	s.blobs[digest] = append(s.uploads[uploadID], payload...)
	delete(s.uploads, uploadID)
	w.WriteHeader(http.StatusCreated)
}
