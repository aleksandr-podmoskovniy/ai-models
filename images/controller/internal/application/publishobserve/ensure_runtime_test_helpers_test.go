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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fakeSourceWorkerRuntime struct {
	handle *publicationports.SourceWorkerHandle
	err    error
	calls  int
}

func (f *fakeSourceWorkerRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.SourceWorkerHandle, bool, error) {
	f.calls++
	return f.handle, false, f.err
}

type fakeUploadSessionRuntime struct {
	handle *publicationports.UploadSessionHandle
	err    error
	calls  int
}

func (f *fakeUploadSessionRuntime) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.UploadSessionHandle, bool, error) {
	f.calls++
	return f.handle, false, f.err
}

func (f *fakeUploadSessionRuntime) MarkPublishing(context.Context, types.UID) error {
	return nil
}

func (f *fakeUploadSessionRuntime) MarkCompleted(context.Context, types.UID) error {
	return nil
}

func (f *fakeUploadSessionRuntime) MarkFailed(context.Context, types.UID, string) error {
	return nil
}

func uploadRequest() publicationports.Request {
	request := testRequest()
	request.Owner.Name = "deepseek-r1-upload"
	request.Identity.Name = "deepseek-r1-upload"
	request.Spec.Source = modelsv1alpha1.ModelSourceSpec{
		Upload: &modelsv1alpha1.UploadModelSource{},
	}
	return request
}
