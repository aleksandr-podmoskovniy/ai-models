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
	"testing"

	"github.com/deckhouse/ai-models/dmcr/internal/sealedblob"
)

func TestServiceCompleteVerifiesAlreadyCompletedObject(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.completeErr = errors.New("multipart upload is already completed")
	startPayload, claims := h.startWithClaims(t)
	parts := singlePart()
	digest := digestForParts(parts)
	h.backend.objects[claims.ObjectKey] = payloadForParts(parts)

	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 8)
	expectStatus(t, completeResp, http.StatusOK)
	if got := h.backend.readerCalls; got != 0 {
		t.Fatalf("Reader() call count = %d, want 0 for default client-asserted path", got)
	}
	linkKey, err := RepositoryBlobLinkObjectKey("/dmcr", testRepository, digest)
	if err != nil {
		t.Fatalf("RepositoryBlobLinkObjectKey() error = %v", err)
	}
	if got, want := string(h.backend.objects[linkKey]), digest; got != want {
		t.Fatalf("link payload = %q, want %q", got, want)
	}
}

func TestServiceCompleteStrictPolicyKeepsPhysicalObjectWhenVerificationReadFails(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.readerErr = errors.New("temporary read failure")
	mustSetVerificationPolicy(t, h.service, VerificationPolicyTrustedBackendOrReread)
	startPayload, claims := h.startWithClaims(t)

	parts := singlePart()
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digestForParts(parts), 8)
	expectStatus(t, completeResp, http.StatusInternalServerError)
	if slices.Contains(h.backend.deleted, claims.ObjectKey) {
		t.Fatalf("physical upload object %q was deleted after temporary verification failure", claims.ObjectKey)
	}
	if _, exists := h.backend.objects[claims.ObjectKey]; !exists {
		t.Fatalf("physical upload object %q does not exist after temporary verification failure", claims.ObjectKey)
	}
}

func TestServiceCompleteStrictPolicyFallsBackWhenBackendDigestLookupFails(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.attributesErr = errors.New("checksum metadata is not supported")
	mustSetVerificationPolicy(t, h.service, VerificationPolicyTrustedBackendOrReread)
	startPayload := h.start(t)
	parts := standardParts()
	digest := digestForParts(parts)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 12)
	expectStatus(t, completeResp, http.StatusOK)
	if got := h.backend.readerCalls; got != 1 {
		t.Fatalf("Reader() call count = %d, want 1 for strict reread policy", got)
	}
}

func TestServiceCompleteCleansUpSealedObjectsWhenLinkWriteFails(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.putErr = errors.New("link write failed")
	h.backend.putErrPathSuffix = "/link"
	startPayload, claims := h.startWithClaims(t)
	parts := singlePart()
	digest := digestForParts(parts)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 8)
	expectStatus(t, completeResp, http.StatusInternalServerError)

	blobKey, err := BlobDataObjectKey("/dmcr", digest)
	if err != nil {
		t.Fatalf("BlobDataObjectKey() error = %v", err)
	}
	expectedDeleted := []string{claims.ObjectKey, sealedblob.MetadataPath(blobKey)}
	for _, expectedPath := range expectedDeleted {
		if !slices.Contains(h.backend.deleted, expectedPath) {
			t.Fatalf("DeleteObject() did not remove %q, deleted = %#v", expectedPath, h.backend.deleted)
		}
		if _, exists := h.backend.objects[expectedPath]; exists {
			t.Fatalf("object %q still exists after cleanup", expectedPath)
		}
	}
}
