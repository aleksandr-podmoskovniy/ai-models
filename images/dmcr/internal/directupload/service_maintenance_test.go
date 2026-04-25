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
	"net/http"
	"testing"
)

type maintenanceCheckerFunc func(context.Context) (bool, error)

func (f maintenanceCheckerFunc) Active(ctx context.Context) (bool, error) {
	return f(ctx)
}

func TestServiceMaintenanceGateBlocksMutationsAndAllowsParts(t *testing.T) {
	t.Parallel()

	h := newServiceHarness(t)
	started := h.start(t)
	h.service.SetMaintenanceChecker(maintenanceCheckerFunc(func(context.Context) (bool, error) {
		return true, nil
	}))

	blocked := postJSON(t, h.server.URL+"/v2/blob-uploads", "writer", "secret", startRequest{Repository: testRepository})
	expectStatus(t, blocked, http.StatusServiceUnavailable)

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/v2/blob-uploads/parts?sessionToken="+started.SessionToken, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	request.SetBasicAuth("writer", "secret")
	allowed, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer allowed.Body.Close()
	expectStatus(t, allowed, http.StatusOK)
}
