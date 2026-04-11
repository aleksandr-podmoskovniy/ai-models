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

package sourceworker

import (
	"context"
	"encoding/base64"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	"github.com/deckhouse/ai-models/controller/internal/application/sourceadmission"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
)

func (s *Service) preflight(
	ctx context.Context,
	request publicationports.Request,
	plan publicationapp.SourceWorkerPlan,
) error {
	headers, err := s.httpProbeHeaders(ctx, plan)
	if err != nil {
		return err
	}

	return sourceadmission.Preflight(ctx, sourceadmission.PreflightInput{
		Owner: ingestadmission.OwnerBinding{
			Kind:      request.Owner.Kind,
			Name:      request.Owner.Name,
			Namespace: request.Owner.Namespace,
			UID:       string(request.Owner.UID),
		},
		Identity:        request.Identity,
		Spec:            request.Spec,
		HTTPHeaders:     headers,
		HTTPSourceProbe: s.httpSourceProbe,
	})
}

func (s *Service) httpProbeHeaders(ctx context.Context, plan publicationapp.SourceWorkerPlan) (map[string]string, error) {
	authRef := sourceAuthSecretRef(plan)
	if authRef == nil || plan.HTTP == nil {
		return nil, nil
	}

	projectedData, err := s.projectedAuthSecretData(ctx, plan, *authRef)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string, 1)
	if authorization, ok := projectedData["authorization"]; ok && len(authorization) > 0 {
		headers["Authorization"] = string(authorization)
		return headers, nil
	}
	username := projectedData["username"]
	password := projectedData["password"]
	if len(username) > 0 && len(password) > 0 {
		token := base64.StdEncoding.EncodeToString([]byte(string(username) + ":" + string(password)))
		headers["Authorization"] = "Basic " + token
	}
	return headers, nil
}

type httpSourcePreflightProber struct{}

func (httpSourcePreflightProber) Probe(ctx context.Context, input sourceadmission.HTTPProbeRequest) (sourceadmission.HTTPProbeResult, error) {
	result, err := sourcefetch.ProbeHTTPSource(ctx, input.URL, input.CABundle, input.Headers)
	if err != nil {
		return sourceadmission.HTTPProbeResult{}, err
	}
	return sourceadmission.HTTPProbeResult{
		FileName:       result.Metadata.Filename,
		ContentType:    result.Metadata.ContentType,
		ContentLength:  result.ContentLength,
		SupportsRanges: result.SupportsRanges,
	}, nil
}
