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
	"log"
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
		"direct upload verification policy started repository=%q objectKey=%q sizeBytes=%d verificationPolicy=%q expectedDigestPresent=%t",
		claims.Repository,
		claims.ObjectKey,
		payload.SizeBytes,
		s.verificationPolicy,
		expectedDigest != "",
	)
	verification, err := s.verifyUploadedObject(request.Context(), claims.ObjectKey, sealedUpload{
		Digest:    expectedDigest,
		SizeBytes: payload.SizeBytes,
	})
	if err != nil {
		log.Printf(
			"direct upload verification failed repository=%q objectKey=%q verificationPolicy=%q error=%v",
			claims.Repository,
			claims.ObjectKey,
			s.verificationPolicy,
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
	log.Printf(
		"direct upload verification source selected repository=%q objectKey=%q verificationPolicy=%q verificationSource=%q declaredDigest=%q declaredSizeBytes=%d digest=%q sizeBytes=%d fallbackReason=%q backendAttributesPresent=%t backendSizeBytes=%d backendChecksumType=%q backendSHA256Present=%t availableChecksums=%q durationMs=%d",
		claims.Repository,
		claims.ObjectKey,
		verification.Policy,
		verification.Source,
		expectedDigest,
		payload.SizeBytes,
		sealed.Digest,
		sealed.SizeBytes,
		verification.FallbackReason,
		verification.BackendAttributesPresent,
		verification.BackendSizeBytes,
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
