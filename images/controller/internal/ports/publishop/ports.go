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
	"context"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SourceWorkerRuntime interface {
	GetOrCreate(ctx context.Context, owner client.Object, request Request) (*SourceWorkerHandle, bool, error)
}

type UploadSessionRuntime interface {
	GetOrCreate(ctx context.Context, owner client.Object, request Request) (*UploadSessionHandle, bool, error)
	MarkPublishing(ctx context.Context, ownerUID types.UID) error
	MarkCompleted(ctx context.Context, ownerUID types.UID) error
	MarkFailed(ctx context.Context, ownerUID types.UID, message string) error
}

type SourceWorkerHandle struct {
	Name               string
	Phase              corev1.PodPhase
	TerminationMessage string
	ProgressReason     modelsv1alpha1.ModelConditionReason
	ProgressMessage    string
	deleteFn           func(context.Context) error
}

func NewSourceWorkerHandle(
	name string,
	phase corev1.PodPhase,
	terminationMessage string,
	progressReason modelsv1alpha1.ModelConditionReason,
	progressMessage string,
	deleteFn func(context.Context) error,
) *SourceWorkerHandle {
	return &SourceWorkerHandle{
		Name:               name,
		Phase:              phase,
		TerminationMessage: terminationMessage,
		ProgressReason:     progressReason,
		ProgressMessage:    progressMessage,
		deleteFn:           deleteFn,
	}
}

func (h *SourceWorkerHandle) Delete(ctx context.Context) error {
	if h == nil || h.deleteFn == nil {
		return nil
	}
	return h.deleteFn(ctx)
}

func (h *SourceWorkerHandle) IsComplete() bool {
	return h != nil && h.Phase == corev1.PodSucceeded
}

func (h *SourceWorkerHandle) IsFailed() bool {
	return h != nil && h.Phase == corev1.PodFailed
}

type UploadSessionHandle struct {
	WorkerName         string
	Phase              corev1.PodPhase
	TerminationMessage string
	Progress           string
	UploadStatus       modelsv1alpha1.ModelUploadStatus
	deleteFn           func(context.Context) error
}

func NewUploadSessionHandle(
	workerName string,
	phase corev1.PodPhase,
	terminationMessage string,
	progress string,
	uploadStatus modelsv1alpha1.ModelUploadStatus,
	deleteFn func(context.Context) error,
) *UploadSessionHandle {
	return &UploadSessionHandle{
		WorkerName:         workerName,
		Phase:              phase,
		TerminationMessage: terminationMessage,
		Progress:           progress,
		UploadStatus:       uploadStatus,
		deleteFn:           deleteFn,
	}
}

func (h *UploadSessionHandle) Delete(ctx context.Context) error {
	if h == nil || h.deleteFn == nil {
		return nil
	}
	return h.deleteFn(ctx)
}

func (h *UploadSessionHandle) IsComplete() bool {
	return h != nil && h.Phase == corev1.PodSucceeded
}

func (h *UploadSessionHandle) IsFailed() bool {
	return h != nil && h.Phase == corev1.PodFailed
}
