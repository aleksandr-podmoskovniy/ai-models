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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

const DefaultBlobPartSizeBytes int64 = 8 << 20
const DefaultSessionTTL = 24 * time.Hour

type Service struct {
	backend       Backend
	authUsername  string
	authPassword  string
	tokenSecret   []byte
	rootDirectory string
	partSizeBytes int64
	sessionTTL    time.Duration
	now           func() time.Time
}

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

type completeRequest struct {
	SessionToken string         `json:"sessionToken"`
	Digest       string         `json:"digest,omitempty"`
	SizeBytes    int64          `json:"sizeBytes"`
	Parts        []UploadedPart `json:"parts"`
}

type completeResponse struct {
	OK        bool   `json:"ok"`
	Digest    string `json:"digest"`
	SizeBytes int64  `json:"sizeBytes"`
}

type abortRequest struct {
	SessionToken string `json:"sessionToken"`
}

type sealedUpload struct {
	Digest    string
	SizeBytes int64
}

func NewService(backend Backend, authUsername, authPassword, tokenSecret, rootDirectory string, partSizeBytes int64, sessionTTL time.Duration) (*Service, error) {
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
	if sessionTTL <= 0 {
		sessionTTL = DefaultSessionTTL
	}
	return &Service{
		backend:       backend,
		authUsername:  strings.TrimSpace(authUsername),
		authPassword:  strings.TrimSpace(authPassword),
		tokenSecret:   []byte(strings.TrimSpace(tokenSecret)),
		rootDirectory: strings.TrimSpace(rootDirectory),
		partSizeBytes: partSizeBytes,
		sessionTTL:    sessionTTL,
		now:           time.Now,
	}, nil
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/blob-uploads", s.handleStart)
	mux.HandleFunc("/v2/blob-uploads/presign-part", s.handlePresignPart)
	mux.HandleFunc("/v2/blob-uploads/parts", s.handleListParts)
	mux.HandleFunc("/v2/blob-uploads/complete", s.handleComplete)
	mux.HandleFunc("/v2/blob-uploads/abort", s.handleAbort)
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
	claims, err := s.claims(payload.SessionToken)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	parts, err := normalizeParts(payload.Parts)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	claimedSizeBytes := totalUploadedSize(parts)
	if payload.SizeBytes <= 0 {
		http.Error(writer, "sizeBytes must be positive", http.StatusBadRequest)
		return
	}
	if payload.SizeBytes != claimedSizeBytes {
		http.Error(writer, "sizeBytes must match uploaded parts", http.StatusBadRequest)
		return
	}
	expectedDigest := strings.TrimSpace(payload.Digest)
	completeStarted := s.now()
	log.Printf(
		"direct upload complete started repository=%q objectKey=%q sizeBytes=%d parts=%d",
		claims.Repository,
		claims.ObjectKey,
		payload.SizeBytes,
		len(parts),
	)
	if err := s.completeMultipartUploadOrUseCompletedObject(request.Context(), claims.ObjectKey, claims.UploadID, parts); err != nil {
		log.Printf(
			"direct upload multipart completion failed repository=%q objectKey=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			err,
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	sealStarted := s.now()
	log.Printf(
		"direct upload verification started repository=%q objectKey=%q sizeBytes=%d",
		claims.Repository,
		claims.ObjectKey,
		payload.SizeBytes,
	)
	verification, err := s.verifyUploadedObject(request.Context(), claims.ObjectKey, payload.SizeBytes)
	if err != nil {
		log.Printf(
			"direct upload verification failed repository=%q objectKey=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			err,
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	sealed := verification.Sealed
	if sealed.SizeBytes != payload.SizeBytes {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(
			writer,
			fmt.Sprintf("verified sizeBytes %d does not match expected sizeBytes %d", sealed.SizeBytes, payload.SizeBytes),
			http.StatusConflict,
		)
		return
	}
	if expectedDigest != "" && sealed.Digest != expectedDigest {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(
			writer,
			fmt.Sprintf("verified digest %q does not match expected digest %q", sealed.Digest, expectedDigest),
			http.StatusConflict,
		)
		return
	}
	log.Printf(
		"direct upload verification completed repository=%q objectKey=%q digest=%q sizeBytes=%d method=%q fallbackReason=%q backendChecksumType=%q backendSHA256Present=%t availableChecksums=%q durationMs=%d",
		claims.Repository,
		claims.ObjectKey,
		sealed.Digest,
		sealed.SizeBytes,
		verification.Method,
		verification.FallbackReason,
		verification.BackendChecksumType,
		verification.BackendSHA256Present,
		strings.Join(verification.AvailableChecksums, ","),
		s.now().Sub(sealStarted).Milliseconds(),
	)

	blobKey, err := BlobDataObjectKey(s.rootDirectory, sealed.Digest)
	if err != nil {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	linkKey, err := RepositoryBlobLinkObjectKey(s.rootDirectory, claims.Repository, sealed.Digest)
	if err != nil {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	if exists, err := s.sealedBlobExists(request.Context(), blobKey); err != nil {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		log.Printf(
			"direct upload sealed blob existence check failed repository=%q objectKey=%q blobKey=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			blobKey,
			err,
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	} else if exists {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		if err := s.backend.PutContent(request.Context(), linkKey, []byte(sealed.Digest)); err != nil {
			log.Printf(
				"direct upload repository link write failed repository=%q objectKey=%q blobKey=%q error=%v",
				claims.Repository,
				claims.ObjectKey,
				blobKey,
				err,
			)
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf(
			"direct upload complete finished repository=%q objectKey=%q digest=%q deduplicated=true durationMs=%d",
			claims.Repository,
			claims.ObjectKey,
			sealed.Digest,
			s.now().Sub(completeStarted).Milliseconds(),
		)
		writeJSON(writer, completeResponse{
			OK:        true,
			Digest:    sealed.Digest,
			SizeBytes: sealed.SizeBytes,
		})
		return
	}
	physicalPath := storageDriverPathForObjectKey(s.rootDirectory, claims.ObjectKey)
	if err := s.writeSealedBlobMetadata(request.Context(), blobKey, sealed, physicalPath); err != nil {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		log.Printf(
			"direct upload sealed metadata write failed repository=%q objectKey=%q blobKey=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			blobKey,
			err,
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.backend.PutContent(request.Context(), linkKey, []byte(sealed.Digest)); err != nil {
		if cleanupErr := s.cleanupSealedUpload(request.Context(), blobKey, claims.ObjectKey); cleanupErr != nil {
			log.Printf(
				"direct upload repository link write failed after metadata repository=%q objectKey=%q blobKey=%q error=%v cleanupError=%v",
				claims.Repository,
				claims.ObjectKey,
				blobKey,
				err,
				cleanupErr,
			)
			http.Error(writer, fmt.Sprintf("%s; cleanup failed: %v", err.Error(), cleanupErr), http.StatusInternalServerError)
			return
		}
		log.Printf(
			"direct upload repository link write failed repository=%q objectKey=%q blobKey=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			blobKey,
			err,
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf(
		"direct upload complete finished repository=%q objectKey=%q digest=%q deduplicated=false durationMs=%d",
		claims.Repository,
		claims.ObjectKey,
		sealed.Digest,
		s.now().Sub(completeStarted).Milliseconds(),
	)
	writeJSON(writer, completeResponse{
		OK:        true,
		Digest:    sealed.Digest,
		SizeBytes: sealed.SizeBytes,
	})
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

func (s *Service) sealedBlobExists(ctx context.Context, blobKey string) (bool, error) {
	if exists, err := s.backend.ObjectExists(ctx, blobKey); err != nil || exists {
		return exists, err
	}
	return s.backend.ObjectExists(ctx, sealedblob.MetadataPath(blobKey))
}

func (s *Service) writeSealedBlobMetadata(ctx context.Context, blobKey string, sealed sealedUpload, physicalPath string) error {
	payload, err := sealedblob.Marshal(sealedblob.Metadata{
		Version:      sealedblob.MetadataVersion,
		Digest:       sealed.Digest,
		PhysicalPath: strings.TrimSpace(physicalPath),
		SizeBytes:    sealed.SizeBytes,
	})
	if err != nil {
		return err
	}
	return s.backend.PutContent(ctx, sealedblob.MetadataPath(blobKey), payload)
}

func (s *Service) cleanupSealedUpload(ctx context.Context, blobKey, physicalPath string) error {
	var cleanupErrs []error
	if err := s.backend.DeleteObject(ctx, strings.TrimSpace(physicalPath)); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	if err := s.backend.DeleteObject(ctx, sealedblob.MetadataPath(blobKey)); err != nil {
		cleanupErrs = append(cleanupErrs, err)
	}
	return errors.Join(cleanupErrs...)
}

func totalUploadedSize(parts []UploadedPart) int64 {
	total := int64(0)
	for _, part := range parts {
		total += part.SizeBytes
	}
	return total
}

func newUploadSessionID() (string, error) {
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(randomBytes[:]), nil
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
