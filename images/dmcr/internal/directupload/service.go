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

package directupload

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const DefaultBlobPartSizeBytes int64 = 8 << 20

type Service struct {
	backend       Backend
	authUsername  string
	authPassword  string
	tokenSecret   []byte
	rootDirectory string
	partSizeBytes int64
}

type startRequest struct {
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

type startResponse struct {
	Complete      bool   `json:"complete"`
	SessionToken  string `json:"sessionToken,omitempty"`
	PartSizeBytes int64  `json:"partSizeBytes,omitempty"`
}

type presignPartRequest struct {
	SessionToken string `json:"sessionToken"`
	PartNumber   int    `json:"partNumber"`
}

type presignPartResponse struct {
	URL string `json:"url"`
}

type listPartsResponse struct {
	Parts []UploadedPart `json:"parts"`
}

type completeRequest struct {
	SessionToken string         `json:"sessionToken"`
	Parts        []UploadedPart `json:"parts"`
}

type abortRequest struct {
	SessionToken string `json:"sessionToken"`
}

func NewService(backend Backend, authUsername, authPassword, tokenSecret, rootDirectory string, partSizeBytes int64) (*Service, error) {
	switch {
	case backend == nil:
		return nil, errors.New("direct upload backend must not be nil")
	case strings.TrimSpace(authUsername) == "":
		return nil, errors.New("direct upload auth username must not be empty")
	case strings.TrimSpace(authPassword) == "":
		return nil, errors.New("direct upload auth password must not be empty")
	case strings.TrimSpace(tokenSecret) == "":
		return nil, errors.New("direct upload token secret must not be empty")
	}
	if partSizeBytes <= 0 {
		partSizeBytes = DefaultBlobPartSizeBytes
	}
	return &Service{
		backend:       backend,
		authUsername:  strings.TrimSpace(authUsername),
		authPassword:  strings.TrimSpace(authPassword),
		tokenSecret:   []byte(strings.TrimSpace(tokenSecret)),
		rootDirectory: strings.TrimSpace(rootDirectory),
		partSizeBytes: partSizeBytes,
	}, nil
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/blob-uploads", s.handleStart)
	mux.HandleFunc("/v1/blob-uploads/presign-part", s.handlePresignPart)
	mux.HandleFunc("/v1/blob-uploads/parts", s.handleListParts)
	mux.HandleFunc("/v1/blob-uploads/complete", s.handleComplete)
	mux.HandleFunc("/v1/blob-uploads/abort", s.handleAbort)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := s.authorize(request); err != nil {
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(writer, request)
	})
}

func (s *Service) authorize(request *http.Request) error {
	username, password, ok := request.BasicAuth()
	if !ok || username != s.authUsername || password != s.authPassword {
		return errors.New("unauthorized")
	}
	return nil
}

func (s *Service) handleStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload startRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	blobKey, err := BlobDataObjectKey(s.rootDirectory, payload.Digest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	linkKey, err := RepositoryBlobLinkObjectKey(s.rootDirectory, payload.Repository, payload.Digest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if exists, err := s.backend.BlobExists(request.Context(), blobKey); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	} else if exists {
		if err := s.backend.PutContent(request.Context(), linkKey, []byte(strings.TrimSpace(payload.Digest))); err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(writer, startResponse{Complete: true})
		return
	}
	uploadID, err := s.backend.StartMultipartUpload(request.Context(), blobKey)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	token, err := encodeSessionToken(s.tokenSecret, sessionTokenClaims{
		Repository: strings.Trim(strings.TrimSpace(payload.Repository), "/"),
		Digest:     strings.TrimSpace(payload.Digest),
		UploadID:   strings.TrimSpace(uploadID),
	})
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, startResponse{
		SessionToken:  token,
		PartSizeBytes: s.partSizeBytes,
	})
}

func (s *Service) handlePresignPart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload presignPartRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	claims, blobKey, err := s.claimsAndBlobKey(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.PartNumber <= 0 {
		http.Error(writer, "part number must be positive", http.StatusBadRequest)
		return
	}
	url, err := s.backend.PresignUploadPart(request.Context(), blobKey, claims.UploadID, payload.PartNumber)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, presignPartResponse{URL: url})
}

func (s *Service) handleListParts(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sessionToken := strings.TrimSpace(request.URL.Query().Get("sessionToken"))
	claims, blobKey, err := s.claimsAndBlobKey(sessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	parts, err := s.backend.ListUploadedParts(request.Context(), blobKey, claims.UploadID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	writeJSON(writer, listPartsResponse{Parts: parts})
}

func (s *Service) handleComplete(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload completeRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	claims, blobKey, err := s.claimsAndBlobKey(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	linkKey, err := RepositoryBlobLinkObjectKey(s.rootDirectory, claims.Repository, claims.Digest)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	parts, err := normalizeParts(payload.Parts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.backend.CompleteMultipartUpload(request.Context(), blobKey, claims.UploadID, parts); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.backend.PutContent(request.Context(), linkKey, []byte(claims.Digest)); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, map[string]bool{"ok": true})
}

func (s *Service) handleAbort(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload abortRequest
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	claims, blobKey, err := s.claimsAndBlobKey(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.backend.AbortMultipartUpload(request.Context(), blobKey, claims.UploadID); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, map[string]bool{"ok": true})
}

func (s *Service) claimsAndBlobKey(sessionToken string) (sessionTokenClaims, string, error) {
	claims, err := decodeSessionToken(s.tokenSecret, sessionToken)
	if err != nil {
		return sessionTokenClaims{}, "", err
	}
	blobKey, err := BlobDataObjectKey(s.rootDirectory, claims.Digest)
	if err != nil {
		return sessionTokenClaims{}, "", err
	}
	return claims, blobKey, nil
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}

func normalizeParts(parts []UploadedPart) ([]UploadedPart, error) {
	normalized := make([]UploadedPart, 0, len(parts))
	for _, part := range parts {
		if part.PartNumber <= 0 {
			return nil, fmt.Errorf("part number must be positive")
		}
		if part.SizeBytes <= 0 {
			return nil, fmt.Errorf("part size must be positive")
		}
		if strings.TrimSpace(part.ETag) == "" {
			return nil, fmt.Errorf("part ETag must not be empty")
		}
		normalized = append(normalized, UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       strings.Trim(strings.TrimSpace(part.ETag), "\""),
			SizeBytes:  part.SizeBytes,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].PartNumber < normalized[j].PartNumber
	})
	for index := range normalized {
		expected := index + 1
		if normalized[index].PartNumber != expected {
			return nil, fmt.Errorf("parts must be contiguous from 1")
		}
	}
	return normalized, nil
}

type Server struct {
	httpServer *http.Server
}

func NewServer(listenAddress, tlsCertFile, tlsKeyFile string, service *Service) (*Server, error) {
	switch {
	case service == nil:
		return nil, errors.New("direct upload service must not be nil")
	case strings.TrimSpace(listenAddress) == "":
		return nil, errors.New("direct upload listen address must not be empty")
	case strings.TrimSpace(tlsCertFile) == "":
		return nil, errors.New("direct upload TLS cert file must not be empty")
	case strings.TrimSpace(tlsKeyFile) == "":
		return nil, errors.New("direct upload TLS key file must not be empty")
	}
	return &Server{
		httpServer: &http.Server{
			Addr:    strings.TrimSpace(listenAddress),
			Handler: service.Handler(),
		},
	}, nil
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
