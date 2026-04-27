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
	assertReaderCalls(t, h.backend, 0)
	assertLinkPayload(t, h.backend, testRepository, digest)
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
	assertNotDeleted(t, h.backend, claims.ObjectKey)
	assertObjectExists(t, h.backend, claims.ObjectKey)
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
	assertReaderCalls(t, h.backend, 1)
}

func TestServiceCompleteKeepsSealedObjectsWhenLinkWriteFailsAndRetrySucceeds(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	h.backend.putErr = errors.New("link write failed")
	h.backend.putErrPathSuffix = "/link"
	startPayload, claims := h.startWithClaims(t)
	parts := singlePart()
	digest := digestForParts(parts)
	completeResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 8)
	expectStatus(t, completeResp, http.StatusInternalServerError)

	blobKey := mustBlobDataKey(t, digest)
	assertNotDeleted(t, h.backend, claims.ObjectKey)
	assertObjectExists(t, h.backend, claims.ObjectKey)
	assertObjectExists(t, h.backend, sealedblob.MetadataPath(blobKey))

	h.backend.putErr = nil
	retryResp := h.completeUpload(t, startPayload.SessionToken, parts, digest, 8)
	expectStatus(t, retryResp, http.StatusOK)
	assertNotDeleted(t, h.backend, claims.ObjectKey)
	assertLinkPayload(t, h.backend, testRepository, digest)
}

func TestServiceCompleteKeepsDuplicateUploadWhenDeduplicatedLinkWriteFails(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	firstPayload, firstClaims := h.startWithClaims(t)
	parts := singlePart()
	digest := digestForParts(parts)
	expectStatus(t, h.completeUpload(t, firstPayload.SessionToken, parts, digest, 8), http.StatusOK)

	secondPayload, secondClaims := h.startWithClaims(t)
	h.backend.putErr = errors.New("link write failed")
	h.backend.putErrPathSuffix = "/link"
	expectStatus(t, h.completeUpload(t, secondPayload.SessionToken, parts, digest, 8), http.StatusInternalServerError)
	assertNotDeleted(t, h.backend, secondClaims.ObjectKey)
	assertObjectExists(t, h.backend, secondClaims.ObjectKey)
	assertObjectExists(t, h.backend, firstClaims.ObjectKey)

	h.backend.putErr = nil
	expectStatus(t, h.completeUpload(t, secondPayload.SessionToken, parts, digest, 8), http.StatusOK)
	assertDeleted(t, h.backend, secondClaims.ObjectKey)
	assertObjectExists(t, h.backend, firstClaims.ObjectKey)
}
