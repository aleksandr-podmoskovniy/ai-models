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
	"errors"
	"net/http"
	"strings"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(listenAddress, tlsCertFile, tlsKeyFile string, service *Service) (*Server, error) {
	switch {
	case service == nil:
		return nil, errors.New("direct upload service must not be nil")
	case strings.TrimSpace(listenAddress) == "":
		return nil, errors.New("direct upload listen address must not be empty")
	case strings.TrimSpace(tlsCertFile) == "":
		return nil, errors.New("direct upload TLS cert file must not be empty")
	case strings.TrimSpace(tlsKeyFile) == "":
		return nil, errors.New("direct upload TLS key file must not be empty")
	}
	return &Server{
		httpServer: &http.Server{
			Addr:    strings.TrimSpace(listenAddress),
			Handler: service.Handler(),
		},
	}, nil
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	return s.httpServer.ListenAndServeTLS(certFile, keyFile)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
