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

package sourcefetch

import (
	"context"
	"io"
	"os"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeUploadStaging struct {
	objects map[string][]byte
}

func newFakeUploadStaging() *fakeUploadStaging {
	return &fakeUploadStaging{objects: map[string][]byte{}}
}

func (f *fakeUploadStaging) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeUploadStaging) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeUploadStaging) ListMultipartUploadParts(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	return nil, nil
}

func (f *fakeUploadStaging) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStaging) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeUploadStaging) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{}, nil
}

func (f *fakeUploadStaging) Upload(_ context.Context, input uploadstagingports.UploadInput) error {
	payload, err := io.ReadAll(input.Body)
	if err != nil {
		return err
	}
	f.objects[stagingObjectKey(input.Bucket, input.Key)] = payload
	return nil
}

func (f *fakeUploadStaging) Download(_ context.Context, input uploadstagingports.DownloadInput) error {
	payload := f.objects[stagingObjectKey(input.Bucket, input.Key)]
	return os.WriteFile(input.DestinationPath, payload, 0o644)
}

func (f *fakeUploadStaging) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	delete(f.objects, stagingObjectKey(input.Bucket, input.Key))
	return nil
}

func stagingObjectKey(bucket, key string) string {
	return bucket + "/" + key
}
