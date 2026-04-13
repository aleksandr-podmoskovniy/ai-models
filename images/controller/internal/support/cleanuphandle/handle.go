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

package cleanuphandle

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const AnnotationKey = "ai-models.deckhouse.io/cleanup-handle"

type Kind string

const (
	KindBackendArtifact Kind = "BackendArtifact"
	KindUploadStaging   Kind = "UploadStaging"
)

type ArtifactSnapshot struct {
	Kind   modelsv1alpha1.ModelArtifactLocationKind `json:"kind,omitempty"`
	URI    string                                   `json:"uri,omitempty"`
	Digest string                                   `json:"digest,omitempty"`
}

type BackendArtifactHandle struct {
	Reference                string `json:"reference,omitempty"`
	RepositoryMetadataPrefix string `json:"repositoryMetadataPrefix,omitempty"`
}

type UploadStagingHandle struct {
	Bucket    string `json:"bucket,omitempty"`
	Key       string `json:"key,omitempty"`
	FileName  string `json:"fileName,omitempty"`
	SizeBytes int64  `json:"sizeBytes,omitempty"`
}

type Handle struct {
	Kind          Kind                   `json:"kind,omitempty"`
	Artifact      *ArtifactSnapshot      `json:"artifact,omitempty"`
	Backend       *BackendArtifactHandle `json:"backend,omitempty"`
	UploadStaging *UploadStagingHandle   `json:"uploadStaging,omitempty"`
}

func (h Handle) Validate() error {
	switch h.Kind {
	case KindBackendArtifact:
		if h.Backend == nil {
			return errors.New("backend cleanup handle payload must not be empty")
		}
		if strings.TrimSpace(h.Backend.Reference) == "" {
			return errors.New("backend cleanup handle reference must not be empty")
		}
	case KindUploadStaging:
		if h.UploadStaging == nil {
			return errors.New("upload staging cleanup handle payload must not be empty")
		}
		if strings.TrimSpace(h.UploadStaging.Bucket) == "" {
			return errors.New("upload staging cleanup handle bucket must not be empty")
		}
		if strings.TrimSpace(h.UploadStaging.Key) == "" {
			return errors.New("upload staging cleanup handle key must not be empty")
		}
	default:
		return fmt.Errorf("unsupported cleanup handle kind %q", h.Kind)
	}

	if h.Artifact != nil {
		if strings.TrimSpace(string(h.Artifact.Kind)) == "" {
			return errors.New("cleanup handle artifact kind must not be empty")
		}
		if strings.TrimSpace(h.Artifact.URI) == "" {
			return errors.New("cleanup handle artifact URI must not be empty")
		}
	}

	return nil
}

func Encode(handle Handle) (string, error) {
	if err := handle.Validate(); err != nil {
		return "", err
	}

	payload, err := json.Marshal(handle)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func Decode(raw string) (Handle, error) {
	if strings.TrimSpace(raw) == "" {
		return Handle{}, errors.New("cleanup handle annotation value must not be empty")
	}

	var handle Handle
	if err := json.Unmarshal([]byte(raw), &handle); err != nil {
		return Handle{}, err
	}

	if err := handle.Validate(); err != nil {
		return Handle{}, err
	}

	return handle, nil
}

func FromObject(object metav1.Object) (Handle, bool, error) {
	if object == nil {
		return Handle{}, false, errors.New("object must not be nil")
	}

	value := object.GetAnnotations()[AnnotationKey]
	if strings.TrimSpace(value) == "" {
		return Handle{}, false, nil
	}

	handle, err := Decode(value)
	if err != nil {
		return Handle{}, false, err
	}

	return handle, true, nil
}

func SetOnObject(object metav1.Object, handle Handle) error {
	if object == nil {
		return errors.New("object must not be nil")
	}

	encoded, err := Encode(handle)
	if err != nil {
		return err
	}

	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	annotations[AnnotationKey] = encoded
	object.SetAnnotations(annotations)
	return nil
}
