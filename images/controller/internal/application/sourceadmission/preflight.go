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

package sourceadmission

import (
	"context"
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
)

type HTTPSourceProber interface {
	Probe(ctx context.Context, input HTTPProbeRequest) (HTTPProbeResult, error)
}

type HTTPProbeRequest struct {
	URL      string
	CABundle []byte
	Headers  map[string]string
}

type HTTPProbeResult struct {
	FileName       string
	ContentType    string
	ContentLength  int64
	SupportsRanges bool
}

type PreflightInput struct {
	Owner           ingestadmission.OwnerBinding
	Identity        publicationdata.Identity
	Spec            modelsv1alpha1.ModelSpec
	HTTPHeaders     map[string]string
	HTTPSourceProbe HTTPSourceProber
}

func Preflight(ctx context.Context, input PreflightInput) error {
	if err := ingestadmission.ValidateOwnerBinding(input.Owner, input.Identity); err != nil {
		return err
	}
	if err := ingestadmission.ValidateDeclaredInputFormat(input.Spec.InputFormat); err != nil {
		return err
	}

	sourceType, err := input.Spec.Source.DetectType()
	if err != nil {
		return err
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		return nil
	case modelsv1alpha1.ModelSourceTypeUpload:
		return nil
	case modelsv1alpha1.ModelSourceTypeHTTP:
		if input.HTTPSourceProbe == nil {
			return errors.New("http source preflight requires a probe client")
		}
		probe, err := input.HTTPSourceProbe.Probe(ctx, HTTPProbeRequest{
			URL:      input.Spec.Source.URL,
			CABundle: input.Spec.Source.CABundle,
			Headers:  input.HTTPHeaders,
		})
		if err != nil {
			return err
		}
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(probe.ContentType)), "text/html") {
			return fmt.Errorf("http source %q resolved to content-type %q instead of a model artifact", input.Spec.Source.URL, probe.ContentType)
		}
		return ingestadmission.ValidateRemoteFileName(probe.FileName, input.Spec.InputFormat)
	default:
		return fmt.Errorf("source preflight does not support source type %q", sourceType)
	}
}
