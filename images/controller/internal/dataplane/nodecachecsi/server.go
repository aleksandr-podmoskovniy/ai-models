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

package nodecachecsi

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

type Server struct {
	csi.UnimplementedIdentityServer
	csi.UnimplementedNodeServer

	options Options
}

func NewServer(options Options) (*Server, error) {
	options = normalizeOptions(options)
	if err := validateOptions(options); err != nil {
		return nil, err
	}
	return &Server{options: options}, nil
}

func Run(ctx context.Context, options Options) error {
	server, err := NewServer(options)
	if err != nil {
		return err
	}
	socketPath, err := endpointSocketPath(server.options.Endpoint)
	if err != nil {
		return err
	}
	if err := prepareSocket(socketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen node cache CSI socket: %w", err)
	}

	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, server)
	csi.RegisterNodeServer(grpcServer, server)

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(listener)
	}()

	select {
	case <-ctx.Done():
		grpcServer.GracefulStop()
		if err := <-errCh; err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func endpointSocketPath(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(endpoint, "unix://")
	if endpoint == "" {
		return "", fmt.Errorf("node cache CSI endpoint must not be empty")
	}
	if !filepath.IsAbs(endpoint) {
		return "", fmt.Errorf("node cache CSI endpoint must be an absolute unix socket path")
	}
	return filepath.Clean(endpoint), nil
}

func prepareSocket(socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return fmt.Errorf("create node cache CSI socket directory: %w", err)
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale node cache CSI socket: %w", err)
	}
	return nil
}
