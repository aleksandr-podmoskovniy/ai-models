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
	"github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"k8s.io/apimachinery/pkg/types"
)

type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
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

type Result struct {
	Snapshot      publishedsnapshot.Snapshot `json:"snapshot"`
	CleanupHandle cleanuphandle.Handle       `json:"cleanupHandle"`
}

type Status struct {
	Phase      Phase
	Message    string
	WorkerName string
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
	sourceType, err := r.Spec.Source.DetectType()
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

func (r Result) Validate() error {
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	return r.CleanupHandle.Validate()
}

func validateRequestSource(source modelsv1alpha1.ModelSourceSpec) error {
	sourceType, err := source.DetectType()
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
