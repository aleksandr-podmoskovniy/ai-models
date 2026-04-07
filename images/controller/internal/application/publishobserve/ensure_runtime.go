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

package publishobserve

import (
	"context"
	"fmt"
	"time"

	publicationplan "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnsureRuntimeObservationInput struct {
	Context        context.Context
	Owner          client.Object
	Request        publicationports.Request
	Mode           publicationplan.ExecutionMode
	SourceWorkers  publicationports.SourceWorkerRuntime
	UploadSessions publicationports.UploadSessionRuntime
	Now            time.Time
}

type EnsureRuntimeObservationResult struct {
	Decision RuntimeObservationDecision
	DeleteFn func(context.Context) error
}

func EnsureRuntimeObservation(
	input EnsureRuntimeObservationInput,
) (EnsureRuntimeObservationResult, error) {
	switch input.Mode {
	case publicationplan.ExecutionModeSourceWorker:
		return ensureSourceWorkerObservation(input)
	case publicationplan.ExecutionModeUpload:
		return ensureUploadSessionObservation(input)
	default:
		return EnsureRuntimeObservationResult{}, fmt.Errorf("unsupported publication execution mode %q", input.Mode)
	}
}

func ensureSourceWorkerObservation(
	input EnsureRuntimeObservationInput,
) (EnsureRuntimeObservationResult, error) {
	if input.SourceWorkers == nil {
		return EnsureRuntimeObservationResult{}, fmt.Errorf("source worker runtime must not be nil")
	}

	handle, _, err := input.SourceWorkers.GetOrCreate(input.Context, input.Owner, publicationports.OperationContext{
		Request: input.Request,
	})
	if err != nil {
		return EnsureRuntimeObservationResult{}, err
	}

	decision, err := ObserveSourceWorker(input.Request, handle)
	if err != nil {
		return EnsureRuntimeObservationResult{}, err
	}

	var deleteFn func(context.Context) error
	if handle != nil {
		deleteFn = handle.Delete
	}

	return EnsureRuntimeObservationResult{
		Decision: decision,
		DeleteFn: deleteFn,
	}, nil
}

func ensureUploadSessionObservation(
	input EnsureRuntimeObservationInput,
) (EnsureRuntimeObservationResult, error) {
	if input.UploadSessions == nil {
		return EnsureRuntimeObservationResult{}, fmt.Errorf("upload session runtime must not be nil")
	}

	handle, _, err := input.UploadSessions.GetOrCreate(input.Context, input.Owner, publicationports.OperationContext{
		Request: input.Request,
	})
	if err != nil {
		return EnsureRuntimeObservationResult{}, err
	}

	decision, err := ObserveUploadSession(input.Request, handle, runtimeObservationNow(input.Now))
	if err != nil {
		return EnsureRuntimeObservationResult{}, err
	}

	var deleteFn func(context.Context) error
	if handle != nil {
		deleteFn = handle.Delete
	}

	return EnsureRuntimeObservationResult{
		Decision: decision,
		DeleteFn: deleteFn,
	}, nil
}

func runtimeObservationNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
