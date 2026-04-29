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

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/uploadsessiontoken"
)

func routeFromRequestPath(path string) (sessionID string, pathToken string, action string, ok bool) {
	if !strings.HasPrefix(path, "/v1/upload/") {
		return "", "", "", false
	}
	rest := strings.Trim(strings.TrimPrefix(path, "/v1/upload/"), "/")
	if rest == "" {
		return "", "", "", false
	}
	parts := strings.Split(rest, "/")
	if len(parts) > 3 {
		return "", "", "", false
	}
	sessionID = strings.TrimSpace(parts[0])
	if sessionID == "" {
		return "", "", "", false
	}

	switch len(parts) {
	case 1:
		return sessionID, "", "", true
	case 2:
		if action, ok := uploadAction(parts[1]); ok {
			return sessionID, "", action, true
		}
		pathToken = strings.TrimSpace(parts[1])
		if pathToken == "" {
			return "", "", "", false
		}
		return sessionID, pathToken, "", true
	case 3:
		pathToken = strings.TrimSpace(parts[1])
		if pathToken == "" {
			return "", "", "", false
		}
		if action, ok := uploadAction(parts[2]); ok {
			return sessionID, pathToken, action, true
		}
		return "", "", "", false
	default:
		return "", "", "", false
	}
}

func uploadAction(raw string) (string, bool) {
	action := "/" + strings.Trim(strings.TrimSpace(raw), "/")
	switch action {
	case "/probe", "/init", "/parts", "/complete", "/abort":
		return action, true
	default:
		return "", false
	}
}

func requestToken(request *http.Request, pathToken string) string {
	auth := strings.TrimSpace(request.Header.Get("Authorization"))
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return strings.TrimSpace(pathToken)
}

func authorizeUploadRequest(request *http.Request, pathToken string, expectedTokenHash string) bool {
	expectedTokenHash = strings.TrimSpace(expectedTokenHash)
	if expectedTokenHash == "" {
		return false
	}
	token := requestToken(request, pathToken)
	if token == "" {
		return false
	}
	tokenHash := uploadsessiontoken.Hash(token)
	return subtle.ConstantTimeCompare([]byte(tokenHash), []byte(expectedTokenHash)) == 1
}

func validatePartNumbers(partNumbers []int32) error {
	if len(partNumbers) == 0 {
		return errors.New("partNumbers must not be empty")
	}
	seen := make(map[int32]struct{}, len(partNumbers))
	for _, partNumber := range partNumbers {
		switch {
		case partNumber <= 0:
			return errors.New("partNumbers must be positive")
		case partNumber > maximumMultipartPartCount:
			return fmt.Errorf("partNumbers must not exceed %d", maximumMultipartPartCount)
		}
		if _, exists := seen[partNumber]; exists {
			return errors.New("partNumbers must not contain duplicates")
		}
		seen[partNumber] = struct{}{}
	}
	return nil
}

func normalizeCompletedParts(parts []completedPartRequest) ([]uploadstagingports.CompletedPart, error) {
	if len(parts) == 0 {
		return nil, errors.New("parts must not be empty")
	}
	seen := make(map[int32]struct{}, len(parts))
	result := make([]uploadstagingports.CompletedPart, 0, len(parts))
	for _, part := range parts {
		switch {
		case part.PartNumber <= 0:
			return nil, errors.New("parts.partNumber must be positive")
		case part.PartNumber > maximumMultipartPartCount:
			return nil, fmt.Errorf("parts.partNumber must not exceed %d", maximumMultipartPartCount)
		case strings.TrimSpace(part.ETag) == "":
			return nil, errors.New("parts.etag must not be empty")
		}
		if _, exists := seen[part.PartNumber]; exists {
			return nil, errors.New("parts must not contain duplicate partNumber values")
		}
		seen[part.PartNumber] = struct{}{}
		result = append(result, uploadstagingports.CompletedPart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(part.ETag),
		})
	}
	return result, nil
}

func validateCompleteRequest(state *SessionState, completedParts []uploadstagingports.CompletedPart) error {
	if state == nil {
		return errors.New("upload session multipart state must not be empty")
	}
	if len(state.UploadedParts) == 0 {
		return errors.New("upload session multipart manifest must not be empty")
	}

	uploadedByPart := make(map[int32]UploadedPart, len(state.UploadedParts))
	for _, part := range state.UploadedParts {
		uploadedByPart[part.PartNumber] = part
	}
	for _, part := range completedParts {
		uploaded, found := uploadedByPart[part.PartNumber]
		if !found {
			return fmt.Errorf("multipart manifest is missing uploaded part %d", part.PartNumber)
		}
		if strings.TrimSpace(uploaded.ETag) != strings.TrimSpace(part.ETag) {
			return fmt.Errorf("multipart manifest ETag mismatch for part %d", part.PartNumber)
		}
	}
	return nil
}

func decodeJSON(request *http.Request, destination any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return errors.New("invalid JSON body: multiple JSON values are not allowed")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, statusCode int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	_ = json.NewEncoder(writer).Encode(value)
}

func sanitizedUploadFileName(raw string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if trimmed == "" {
		return "upload.bin"
	}

	base := strings.TrimSpace(filepath.Base(trimmed))
	switch base {
	case "", ".", "..", string(filepath.Separator):
		return "upload.bin"
	}
	if strings.HasPrefix(base, ".") {
		return "upload.bin"
	}
	return base
}

func uploadKey(prefix string, fileName string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "", errors.New("upload staging key prefix must not be empty")
	}
	return prefix + "/" + fileName, nil
}
