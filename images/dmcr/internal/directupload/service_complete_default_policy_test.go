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
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

func TestServiceCompleteWritesRepositoryLinkAndSealedMetadata(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	digest := digestForParts(parts)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusOK)
	completePayload := decodeCompleteResponse(t, completeResp)
	if completePayload.Digest != digest {
		t.Fatalf("complete digest = %q, want %q", completePayload.Digest, digest)
	}
	if completePayload.SizeBytes != 12 {
		t.Fatalf("complete sizeBytes = %d, want 12", completePayload.SizeBytes)
	}

	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", "ai-models/catalog/model", digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(h.backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}

	blobKey, err := BlobDataObjectKey("/dmcr", digest)
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	if _, exists := h.backend.objects[blobKey]; exists {
		t.Fatalf("canonical blob object %q exists, want sealed metadata only", blobKey)
	}

	metadataPayload, exists := h.backend.objects[sealedblob.MetadataPath(blobKey)]
	if !exists {
		t.Fatalf("sealed metadata %q was not written", sealedblob.MetadataPath(blobKey))
	}
	metadata, err := sealedblob.Unmarshal(metadataPayload)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if metadata.Digest != digest {
		t.Fatalf("metadata.Digest = %q, want %q", metadata.Digest, digest)
	}
	expectedPhysicalPath := storageDriverPathForObjectKey("/dmcr", claims.ObjectKey)
	if metadata.PhysicalPath != expectedPhysicalPath {
		t.Fatalf("metadata.PhysicalPath = %q, want %q", metadata.PhysicalPath, expectedPhysicalPath)
	}
	if metadata.SizeBytes != 12 {
		t.Fatalf("metadata.SizeBytes = %d, want %d", metadata.SizeBytes, 12)
	}
	if _, exists := h.backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteUsesTrustedBackendDigestWithoutReadingObject(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	digest := digestForParts(parts)
	h.backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: digest,
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}

	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusOK)
	if got := h.backend.attributesCalls; got != 1 {
		t.Fatalf("ObjectAttributes() call count = %d, want 1", got)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for trusted backend digest path", got)
	}
}

func TestServiceCompleteTrustsClientDigestWhenBackendDigestLookupFailsByDefault(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.attributesErr = errors.New("checksum metadata is not supported")
	startPayload := h.start(t)
	parts := standardParts()
	digest := digestForParts(parts)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusOK)
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteTrustsClientDigestWhenTrustedBackendDigestIsMalformedByDefault(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	digest := digestForParts(parts)
	h.backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: "sha256:not-a-valid-digest",
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusOK)
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
}

func TestServiceCompleteRejectsTrustedBackendSizeMismatch(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	digest := digestForParts(parts)
	h.backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     11,
		TrustedFullObjectSHA256Digest: digest,
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusConflict)
	if !slices.Contains(h.backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, h.backend.deleted)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", testRepository, digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if _, exists := h.backend.objects[linkKey]; exists {
		t.Fatalf("repository link %q exists after trusted size mismatch", linkKey)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 when trusted size mismatches", got)
	}
}

func TestServiceCompleteRejectsTrustedBackendDigestMismatch(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	expectedDigest := digestForParts(parts)
	h.backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                     12,
		TrustedFullObjectSHA256Digest: "sha256:" + strings.Repeat("f", 64),
		ReportedChecksumType:          checksumTypeFullObject,
		SHA256ChecksumPresent:         true,
		AvailableChecksumAlgorithms:   []string{"SHA256"},
	}
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, expectedDigest, 12)
	expectStatus(t, completeResp, http.StatusConflict)
	if !slices.Contains(h.backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, h.backend.deleted)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", testRepository, expectedDigest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if _, exists := h.backend.objects[linkKey]; exists {
		t.Fatalf("repository link %q exists after trusted digest mismatch", linkKey)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 when trusted digest mismatches", got)
	}
}

func TestServiceCompleteTrustsExpectedDigestWithoutTrustedBackendChecksumByDefault(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := singlePart()
	trustedDigest := "sha256:" + strings.Repeat("f", 64)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, trustedDigest, 8)
	expectStatus(t, completeResp, http.StatusOK)
	if slices.Contains(h.backend.deleted, claims.ObjectKey) {
		t.Fatalf("physical upload object %q was deleted, deleted = %#v", claims.ObjectKey, h.backend.deleted)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", testRepository, trustedDigest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(h.backend.objects[linkKey]), trustedDigest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
	if _, exists := h.backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
}

func TestServiceCompleteComputesDigestWithoutClientDigest(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := singlePart()
	completeResp := h.complete(t, completeRequest{
		SessionToken: startPayload.SessionToken,
		SizeBytes:    8,
		Parts:        parts,
	})
	expectStatus(t, completeResp, http.StatusOK)
	completePayload := decodeCompleteResponse(t, completeResp)
	digest := digestForParts(parts)
	if completePayload.Digest != digest {
		t.Fatalf("complete digest = %q, want %q", completePayload.Digest, digest)
	}
	if completePayload.SizeBytes != 8 {
		t.Fatalf("complete sizeBytes = %d, want 8", completePayload.SizeBytes)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", testRepository, digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(h.backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
	if _, exists := h.backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist", claims.ObjectKey)
	}
	if got := h.backend.readerCalls; got != 1 {
		t.Fatalf("Reader() call count = %d, want 1 when client digest is absent", got)
	}
}

func TestServiceCompleteRejectsBackendSizeMismatchWithoutChecksumByDefault(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startPayload, claims := h.startWithClaims(t)
	parts := standardParts()
	digest := digestForParts(parts)
	h.backend.attributes[claims.ObjectKey] = ObjectAttributes{
		SizeBytes:                   11,
		ReportedChecksumType:        "",
		SHA256ChecksumPresent:       false,
		AvailableChecksumAlgorithms: nil,
	}
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusConflict)
	if !slices.Contains(h.backend.deleted, claims.ObjectKey) {
		t.Fatalf("expected physical upload object %q to be deleted, deleted = %#v", claims.ObjectKey, h.backend.deleted)
	}
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 without reread in default policy", got)
	}
}
