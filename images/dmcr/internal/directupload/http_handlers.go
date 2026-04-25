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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
)

type startRequest struct {
	Repository string `json:"repository"`
}

type startResponse struct {
	SessionToken  string `json:"sessionToken"`
	PartSizeBytes int64  `json:"partSizeBytes"`
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

type abortRequest struct {
	SessionToken string `json:"sessionToken"`
}

const healthPath = "/healthz"

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(healthPath, handleHealth)
	mux.HandleFunc("/v2/blob-uploads", s.handleStart)
	mux.HandleFunc("/v2/blob-uploads/presign-part", s.handlePresignPart)
	mux.HandleFunc("/v2/blob-uploads/parts", s.handleListParts)
	mux.HandleFunc("/v2/blob-uploads/complete", s.handleComplete)
	mux.HandleFunc("/v2/blob-uploads/abort", s.handleAbort)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != healthPath {
			if err := s.authorize(request); err != nil {
				http.Error(writer, err.Error(), http.StatusUnauthorized)
				return
			}
			if isMutationRequest(request) && maintenance.RejectWriteIfActive(writer, request, s.maintenanceChecker) {
				return
			}
		}
		mux.ServeHTTP(writer, request)
	})
}

func handleHealth(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *Service) authorize(request *http.Request) error {
	username, password, ok := request.BasicAuth()
	if !ok || username != s.authUsername || password != s.authPassword {
		return errors.New("unauthorized")
	}
	return nil
}

func isMutationRequest(request *http.Request) bool {
	if request == nil || !strings.HasPrefix(request.URL.Path, "/v2/blob-uploads") {
		return false
	}
	return request.Method != http.MethodGet && request.Method != http.MethodHead
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
	repository := strings.Trim(strings.TrimSpace(payload.Repository), "/")
	if repository == "" {
		http.Error(writer, "repository must not be empty", http.StatusBadRequest)
		return
	}
	sessionID, err := newUploadSessionID()
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	objectKey, err := UploadSessionObjectKey(s.rootDirectory, sessionID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	uploadID, err := s.backend.StartMultipartUpload(request.Context(), objectKey)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	token, err := encodeSessionToken(s.tokenSecret, sessionTokenClaims{
		Repository:  repository,
		ObjectKey:   objectKey,
		UploadID:    strings.TrimSpace(uploadID),
		ExpiresUnix: s.now().Add(s.sessionTTL).Unix(),
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
	claims, err := s.claims(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.PartNumber <= 0 {
		http.Error(writer, "part number must be positive", http.StatusBadRequest)
		return
	}
	url, err := s.backend.PresignUploadPart(request.Context(), claims.ObjectKey, claims.UploadID, payload.PartNumber)
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
	claims, err := s.claims(sessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	parts, err := s.backend.ListUploadedParts(request.Context(), claims.ObjectKey, claims.UploadID)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})
	writeJSON(writer, listPartsResponse{Parts: parts})
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
	claims, err := s.claims(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.backend.AbortMultipartUpload(request.Context(), claims.ObjectKey, claims.UploadID); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(writer, map[string]bool{"ok": true})
}

func (s *Service) claims(sessionToken string) (sessionTokenClaims, error) {
	claims, err := decodeSessionToken(s.tokenSecret, sessionToken)
	if err != nil {
		return sessionTokenClaims{}, err
	}
	if strings.TrimSpace(claims.ObjectKey) == "" {
		return sessionTokenClaims{}, errors.New("direct upload session token is missing object key")
	}
	if claims.expiredAt(s.now()) {
		return sessionTokenClaims{}, errors.New("direct upload session token expired")
	}
	return claims, nil
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}

func newUploadSessionID() (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes[:]), nil
}
