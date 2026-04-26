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
	"errors"
	"path/filepath"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
)

type Options struct {
	DriverName string
	NodeID     string
	CacheRoot  string
	Endpoint   string
	Mounter    Mounter
	Authorizer PublishAuthorizer
}

type PublishAuthorizer interface {
	AllowPublish(ctx context.Context, attributes map[string]string, digest string) (bool, error)
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.DriverName) == "" {
		options.DriverName = nodecache.CSIDriverName
	}
	if strings.TrimSpace(options.Endpoint) == "" {
		options.Endpoint = nodecache.CSIContainerSocketPath
	}
	if options.Mounter == nil {
		options.Mounter = defaultMounter{}
	}
	options.DriverName = strings.TrimSpace(options.DriverName)
	options.NodeID = strings.TrimSpace(options.NodeID)
	options.CacheRoot = filepath.Clean(strings.TrimSpace(options.CacheRoot))
	options.Endpoint = strings.TrimSpace(options.Endpoint)
	return options
}

func validateOptions(options Options) error {
	switch {
	case strings.TrimSpace(options.DriverName) == "":
		return errors.New("node cache CSI driver name must not be empty")
	case strings.TrimSpace(options.NodeID) == "":
		return errors.New("node cache CSI node ID must not be empty")
	case options.CacheRoot == "" || options.CacheRoot == ".":
		return errors.New("node cache CSI cache root must not be empty")
	case strings.TrimSpace(options.Endpoint) == "":
		return errors.New("node cache CSI endpoint must not be empty")
	case options.Mounter == nil:
		return errors.New("node cache CSI mounter must not be nil")
	default:
		return nil
	}
}
