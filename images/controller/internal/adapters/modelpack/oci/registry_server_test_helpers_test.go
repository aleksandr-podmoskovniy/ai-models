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

package oci

import (
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type writableRegistryServer struct {
	server        *httptest.Server
	state         *writableRegistryState
	getPatchCount func() int
}

func (s *writableRegistryServer) Close() {
	s.server.Close()
}

func (s *writableRegistryServer) patchCount() int {
	return s.getPatchCount()
}

func newWritableRegistryServer(t *testing.T) (*writableRegistryServer, modelpackports.RegistryAuth) {
	return newWritableRegistryServerWithOptions(t, false)
}

func newWritableRegistryServerWithOptions(t *testing.T, interruptFirstPatch bool) (*writableRegistryServer, modelpackports.RegistryAuth) {
	t.Helper()

	state := newWritableRegistryState(t, interruptFirstPatch)
	server := httptest.NewTLSServer(http.HandlerFunc(state.serve))

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	return &writableRegistryServer{
			server: server,
			state:  state,
			getPatchCount: func() int {
				return state.patches
			},
		}, modelpackports.RegistryAuth{
			Username: "writer",
			Password: "secret",
			CAFile:   writeTempFile(t, certPEM),
		}
}
