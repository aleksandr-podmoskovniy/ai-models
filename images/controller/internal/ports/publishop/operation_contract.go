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

package publishop

import (
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
	"github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"k8s.io/apimachinery/pkg/types"
)

type Owner struct {
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
	UID       types.UID `json:"uid"`
}

type Request struct {
	Owner       Owner                              `json:"owner"`
	Identity    publishedsnapshot.Identity         `json:"identity"`
	Spec        modelsv1alpha1.ModelSpec           `json:"spec"`
	UploadStage *cleanuphandle.UploadStagingHandle `json:"uploadStage,omitempty"`
}

func (r Request) Validate() error {
	if strings.TrimSpace(r.Owner.Kind) == "" {
		return errors.New("publication operation owner kind must not be empty")
	}
	if strings.TrimSpace(r.Owner.Name) == "" {
		return errors.New("publication operation owner name must not be empty")
	}
	if strings.TrimSpace(string(r.Owner.UID)) == "" {
		return errors.New("publication operation owner UID must not be empty")
	}
	if err := r.Identity.Validate(); err != nil {
		return err
	}
	sourceType, err := modelsource.DetectType(r.Spec.Source)
	if err != nil {
		return err
	}
	if r.UploadStage != nil {
		if sourceType != modelsv1alpha1.ModelSourceTypeUpload {
			return errors.New("publication operation upload stage is only supported for upload source")
		}
		if err := (cleanuphandle.Handle{
			Kind:          cleanuphandle.KindUploadStaging,
			UploadStaging: r.UploadStage,
		}).Validate(); err != nil {
			return err
		}
	}
	return validateRequestSource(r.Spec.Source)
}

func validateRequestSource(source modelsv1alpha1.ModelSourceSpec) error {
	sourceType, err := modelsource.DetectType(source)
	if err != nil {
		return err
	}

	switch sourceType {
	case modelsv1alpha1.ModelSourceTypeUpload:
		if source.Upload == nil {
			return errors.New("publication operation upload source must not be empty")
		}
	case modelsv1alpha1.ModelSourceTypeHuggingFace:
		if strings.TrimSpace(source.URL) == "" {
			return errors.New("publication operation source url must not be empty")
		}
	default:
		return fmt.Errorf("publication operation does not support source type %q", sourceType)
	}

	return nil
}
