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

package publicationops

import (
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/artifactbackend"
	publicationdomain "github.com/deckhouse/ai-models/controller/internal/domain/publication"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publication"
	"github.com/deckhouse/ai-models/controller/internal/publication"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	corev1 "k8s.io/api/core/v1"
)

func publicationSuccessFromWorkerResult(
	rawResult string,
	request publicationports.Request,
) (*publicationdomain.PublicationSuccess, error) {
	backendResult, err := artifactbackend.DecodeResult(rawResult)
	if err != nil {
		return nil, err
	}

	return publicationSuccessFromSnapshot(publication.Snapshot{
		Identity: request.Identity,
		Source:   backendResult.Source,
		Artifact: backendResult.Artifact,
		Resolved: backendResult.Resolved,
		Result: publication.Result{
			State: "Published",
			Ready: true,
		},
	}, backendResult.CleanupHandle), nil
}

func publicationSuccessFromSnapshot(
	snapshot publication.Snapshot,
	cleanup cleanuphandle.Handle,
) *publicationdomain.PublicationSuccess {
	return &publicationdomain.PublicationSuccess{
		Snapshot:      snapshot,
		CleanupHandle: cleanup,
	}
}

func workerFailureMessage(operation *corev1.ConfigMap, fallback string) string {
	message := strings.TrimSpace(WorkerFailureFromConfigMap(operation))
	if message != "" {
		return message
	}

	return fallback
}
