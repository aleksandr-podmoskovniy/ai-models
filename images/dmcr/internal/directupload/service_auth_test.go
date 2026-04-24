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
	"net/http"
	"testing"
	"time"
)

func TestServiceStartReturnsSessionToken(t *testing.T) {
	t.Parallel()

	response := postJSON(t, newServiceHarness(t).server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{Repository: testRepository})
	expectStatus(t, response, http.StatusOK)
	payload := decodeStartResponse(t, response)
	if payload.SessionToken == "" {
		t.Fatal("SessionToken = empty, want non-empty token")
	}
}

func TestServiceRejectsWrongAuth(t *testing.T) {
	t.Parallel()

	response := postJSON(t, newServiceHarness(t).server.URL+"/v2/blob-uploads", "writer", "wrong", startRequest{Repository: testRepository})
	expectStatus(t, response, http.StatusUnauthorized)
}

func TestServiceHealthDoesNotRequireAuth(t *testing.T) {
	t.Parallel()

	response, err := http.Get(newServiceHarness(t).server.URL + healthPath)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", healthPath, err)
	}
	defer response.Body.Close()
	expectStatus(t, response, http.StatusNoContent)
}

func TestServiceRejectsExpiredSessionToken(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	startedAt := time.Unix(1_700_000_000, 0)
	h.service.now = func() time.Time { return startedAt }
	startPayload := h.start(t)

	h.service.now = func() time.Time { return startedAt.Add(2 * time.Hour) }

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/v2/blob-uploads/parts?sessionToken="+startPayload.SessionToken, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.SetBasicAuth("writer", "secret")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	expectStatus(t, response, http.StatusBadRequest)
}
