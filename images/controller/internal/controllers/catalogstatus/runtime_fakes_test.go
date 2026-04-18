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

package catalogstatus

import (
	"context"

	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeSourceWorkerRuntime struct {
	handle  *publicationports.SourceWorkerHandle
	handles []*publicationports.SourceWorkerHandle
	err     error
	calls   int
}

func (f *fakeSourceWorkerRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.SourceWorkerHandle, bool, error) {
	f.calls++
	if len(f.handles) == 0 {
		return f.handle, false, f.err
	}
	index := f.calls - 1
	if index < 0 {
		index = 0
	}
	if index >= len(f.handles) {
		index = len(f.handles) - 1
	}
	return f.handles[index], false, f.err
}

type fakeUploadSessionRuntime struct {
	handle              *publicationports.UploadSessionHandle
	err                 error
	calls               int
	markPublishingCalls int
	markCompletedCalls  int
	markFailedCalls     int
	failedMessages      []string
}

func (f *fakeUploadSessionRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.UploadSessionHandle, bool, error) {
	f.calls++
	return f.handle, false, f.err
}

func (f *fakeUploadSessionRuntime) MarkPublishing(context.Context, types.UID) error {
	f.markPublishingCalls++
	return f.err
}

func (f *fakeUploadSessionRuntime) MarkCompleted(context.Context, types.UID) error {
	f.markCompletedCalls++
	return f.err
}

func (f *fakeUploadSessionRuntime) MarkFailed(_ context.Context, _ types.UID, message string) error {
	f.markFailedCalls++
	f.failedMessages = append(f.failedMessages, message)
	return f.err
}

type fakeAuditSink struct {
	records []auditsink.Record
}

func (f *fakeAuditSink) Record(_ client.Object, record auditsink.Record) {
	f.records = append(f.records, record)
}
