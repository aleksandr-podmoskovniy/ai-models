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

package uploadsession

import (
	"errors"
	"strings"
	"time"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

const (
	uploadPort      = 8444
	defaultTokenTTL = 30 * time.Minute
)

type Options struct {
	Runtime       RuntimeOptions
	Gateway       GatewayOptions
	StagingBucket string
	StagingClient uploadstagingports.MultipartStager
	TokenTTL      time.Duration
}

type RuntimeOptions struct {
	Namespace           string
	OCIRepositoryPrefix string
}

type GatewayOptions struct {
	ServiceName string
	PublicHost  string
}

func normalizeOptions(options Options) Options {
	options.Runtime.Namespace = strings.TrimSpace(options.Runtime.Namespace)
	options.Runtime.OCIRepositoryPrefix = strings.TrimSpace(options.Runtime.OCIRepositoryPrefix)
	options.Gateway.ServiceName = strings.TrimSpace(options.Gateway.ServiceName)
	options.Gateway.PublicHost = strings.TrimSpace(options.Gateway.PublicHost)
	options.StagingBucket = strings.TrimSpace(options.StagingBucket)
	if options.TokenTTL <= 0 {
		options.TokenTTL = defaultTokenTTL
	}
	return options
}

func (o Options) Validate() error {
	switch {
	case o.Runtime.Namespace == "":
		return errors.New("upload session runtime namespace must not be empty")
	case o.Runtime.OCIRepositoryPrefix == "":
		return errors.New("upload session OCI repository prefix must not be empty")
	case o.Gateway.ServiceName == "":
		return errors.New("upload session gateway service name must not be empty")
	case o.StagingBucket == "" && o.StagingClient != nil:
		return errors.New("upload session staging bucket must not be empty when staging client is configured")
	case o.StagingBucket != "" && o.StagingClient == nil:
		return errors.New("upload session staging client must not be nil when staging bucket is configured")
	case o.TokenTTL <= 0:
		return errors.New("upload session token ttl must be positive")
	default:
		return nil
	}
}
