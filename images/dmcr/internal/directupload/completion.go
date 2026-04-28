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
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
)

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

type sealedUpload struct {
	Digest    string
	SizeBytes int64
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
	slog.Default().Info(
		"direct upload complete started",
		slog.String("repository", claims.Repository),
		slog.String("objectKey", claims.ObjectKey),
		slog.Int64("sizeBytes", payload.SizeBytes),
		slog.Int("partCount", len(parts)),
	)
	if err := s.completeMultipartUploadOrUseCompletedObject(request.Context(), claims.ObjectKey, claims.UploadID, parts); err != nil {
		slog.Default().Error(
			"direct upload multipart completion failed",
			slog.String("repository", claims.Repository),
			slog.String("objectKey", claims.ObjectKey),
			slog.Any("error", err),
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

	sealStarted := s.now()
	slog.Default().Info(
		"direct upload verification policy started",
		slog.String("repository", claims.Repository),
		slog.String("objectKey", claims.ObjectKey),
		slog.Int64("sizeBytes", payload.SizeBytes),
		slog.String("verificationPolicy", string(s.verificationPolicy)),
		slog.Bool("expectedDigestPresent", expectedDigest != ""),
	)
	verification, err := s.verifyUploadedObject(request.Context(), claims.ObjectKey, sealedUpload{
		Digest:    expectedDigest,
		SizeBytes: payload.SizeBytes,
	})
	if err != nil {
		slog.Default().Error(
			"direct upload verification failed",
			slog.String("repository", claims.Repository),
			slog.String("objectKey", claims.ObjectKey),
			slog.String("verificationPolicy", string(s.verificationPolicy)),
			slog.Any("error", err),
		)
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	sealed := verification.Sealed
	if sealed.SizeBytes != payload.SizeBytes {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(
			writer,
			fmt.Sprintf("resolved sizeBytes %d does not match expected sizeBytes %d", sealed.SizeBytes, payload.SizeBytes),
			http.StatusConflict,
		)
		return
	}
	if expectedDigest != "" && sealed.Digest != expectedDigest {
		_ = s.backend.DeleteObject(request.Context(), claims.ObjectKey)
		http.Error(
			writer,
			fmt.Sprintf("resolved digest %q does not match expected digest %q", sealed.Digest, expectedDigest),
			http.StatusConflict,
		)
		return
	}
	slog.Default().Info(
		"direct upload verification source selected",
		slog.String("repository", claims.Repository),
		slog.String("objectKey", claims.ObjectKey),
		slog.String("verificationPolicy", string(verification.Policy)),
		slog.String("verificationSource", string(verification.Source)),
		slog.String("declaredDigest", expectedDigest),
		slog.Int64("declaredSizeBytes", payload.SizeBytes),
		slog.String("artifactDigest", sealed.Digest),
		slog.Int64("sizeBytes", sealed.SizeBytes),
		slog.String("fallbackReason", string(verification.FallbackReason)),
		slog.Bool("backendAttributesPresent", verification.BackendAttributesPresent),
		slog.Int64("backendSizeBytes", verification.BackendSizeBytes),
		slog.String("backendChecksumType", verification.BackendChecksumType),
		slog.Bool("backendSHA256Present", verification.BackendSHA256Present),
		slog.String("availableChecksums", strings.Join(verification.AvailableChecksums, ",")),
		slog.Int64("durationMs", s.now().Sub(sealStarted).Milliseconds()),
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
	deduplicated, err := s.finalizeVerifiedUpload(request.Context(), claims, blobKey, linkKey, sealed)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Default().Info(
		"direct upload complete finished",
		slog.String("repository", claims.Repository),
		slog.String("objectKey", claims.ObjectKey),
		slog.String("artifactDigest", sealed.Digest),
		slog.Bool("deduplicated", deduplicated),
		slog.Int64("durationMs", s.now().Sub(completeStarted).Milliseconds()),
	)
	writeJSON(writer, completeResponse{
		OK:        true,
		Digest:    sealed.Digest,
		SizeBytes: sealed.SizeBytes,
	})
}

func (s *Service) finalizeVerifiedUpload(ctx context.Context, claims sessionTokenClaims, blobKey, linkKey string, sealed sealedUpload) (bool, error) {
	uploadedPhysicalPath := storageDriverPathForObjectKey(s.rootDirectory, claims.ObjectKey)
	state, err := s.sealedBlobState(ctx, blobKey, sealed, uploadedPhysicalPath)
	if err != nil {
		slog.Default().Error(
			"direct upload sealed blob state check failed",
			slog.String("repository", claims.Repository),
			slog.String("objectKey", claims.ObjectKey),
			slog.String("blobKey", blobKey),
			slog.Any("error", err),
		)
		return false, err
	}
	if state.exists {
		if err := s.backend.PutContent(ctx, linkKey, []byte(sealed.Digest)); err != nil {
			slog.Default().Error(
				"direct upload repository link write failed",
				slog.String("repository", claims.Repository),
				slog.String("objectKey", claims.ObjectKey),
				slog.String("blobKey", blobKey),
				slog.Any("error", err),
			)
			return false, err
		}
		if state.deleteUploadedObject {
			if err := s.backend.DeleteObject(ctx, claims.ObjectKey); err != nil {
				slog.Default().Warn(
					"direct upload duplicate object cleanup failed",
					slog.String("repository", claims.Repository),
					slog.String("objectKey", claims.ObjectKey),
					slog.String("blobKey", blobKey),
					slog.Any("error", err),
				)
			}
		}
		return state.deleteUploadedObject, nil
	}
	if err := s.writeSealedBlobMetadata(ctx, blobKey, sealed, uploadedPhysicalPath); err != nil {
		slog.Default().Error(
			"direct upload sealed metadata write failed",
			slog.String("repository", claims.Repository),
			slog.String("objectKey", claims.ObjectKey),
			slog.String("blobKey", blobKey),
			slog.Any("error", err),
		)
		return false, err
	}
	if err := s.backend.PutContent(ctx, linkKey, []byte(sealed.Digest)); err != nil {
		slog.Default().Error(
			"direct upload repository link write failed",
			slog.String("repository", claims.Repository),
			slog.String("objectKey", claims.ObjectKey),
			slog.String("blobKey", blobKey),
			slog.Any("error", err),
		)
		return false, err
	}
	return false, nil
}

func (s *Service) completeMultipartUploadOrUseCompletedObject(ctx context.Context, objectKey, uploadID string, parts []UploadedPart) error {
	objectKey = strings.TrimSpace(objectKey)
	if err := s.backend.CompleteMultipartUpload(ctx, objectKey, strings.TrimSpace(uploadID), parts); err != nil {
		exists, existsErr := s.backend.ObjectExists(ctx, objectKey)
		if existsErr != nil {
			return fmt.Errorf("complete multipart upload failed: %w; completed object existence check failed: %v", err, existsErr)
		}
		if !exists {
			return err
		}
	}
	return nil
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

func totalUploadedSize(parts []UploadedPart) int64 {
	total := int64(0)
	for _, part := range parts {
		total += part.SizeBytes
	}
	return total
}
