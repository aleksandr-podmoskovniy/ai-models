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

package directupload

import (
	"context"
	"io"
)

type UploadedPart struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type Backend interface {
	ObjectExists(ctx context.Context, objectKey string) (bool, error)
	StartMultipartUpload(ctx context.Context, objectKey string) (string, error)
	PresignUploadPart(ctx context.Context, objectKey, uploadID string, partNumber int) (string, error)
	ListUploadedParts(ctx context.Context, objectKey, uploadID string) ([]UploadedPart, error)
	CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, parts []UploadedPart) error
	AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error
	Reader(ctx context.Context, objectKey string, offset int64) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, objectKey string) error
	PutContent(ctx context.Context, objectKey string, payload []byte) error
}
