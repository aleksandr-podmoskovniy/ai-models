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

package maintenance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type checkerFunc func(context.Context) (bool, error)

func (f checkerFunc) Active(ctx context.Context) (bool, error) {
	return f(ctx)
}

func TestRegistryWriteGateBlocksMutatingV2Requests(t *testing.T) {
	handler := RegistryWriteGateHandler(checkerFunc(func(context.Context) (bool, error) {
		return true, nil
	}), http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/v2/repo/blobs/uploads/", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestRegistryWriteGateAllowsReads(t *testing.T) {
	handler := RegistryWriteGateHandler(checkerFunc(func(context.Context) (bool, error) {
		return true, nil
	}), http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v2/repo/manifests/latest", nil))

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
}
