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

package uploadstaging

import (
	"context"
	"io"
	"time"
)

type StartMultipartUploadInput struct {
	Bucket string
	Key    string
}

type StartMultipartUploadOutput struct {
	UploadID string
}

type PresignUploadPartInput struct {
	Bucket     string
	Key        string
	UploadID   string
	PartNumber int32
	Expires    time.Duration
}

type PresignUploadPartOutput struct {
	URL string
}

type UploadedPart struct {
	PartNumber int32
	ETag       string
	SizeBytes  int64
}

type CompletedPart struct {
	PartNumber int32
	ETag       string
}

type ListMultipartUploadPartsInput struct {
	Bucket   string
	Key      string
	UploadID string
}

type CompleteMultipartUploadInput struct {
	Bucket   string
	Key      string
	UploadID string
	Parts    []CompletedPart
}

type AbortMultipartUploadInput struct {
	Bucket   string
	Key      string
	UploadID string
}

type StatInput struct {
	Bucket string
	Key    string
}

type ObjectStat struct {
	SizeBytes int64
	ETag      string
}

type DownloadInput struct {
	Bucket          string
	Key             string
	DestinationPath string
}

type OpenReadInput struct {
	Bucket string
	Key    string
}

type OpenReadRangeInput struct {
	Bucket string
	Key    string
	Offset int64
	Length int64
}

type OpenReadOutput struct {
	Body      io.ReadCloser
	SizeBytes int64
	ETag      string
}

type UploadInput struct {
	Bucket      string
	Key         string
	Body        io.Reader
	ContentType string
}

type DeleteInput struct {
	Bucket string
	Key    string
}

type DeletePrefixInput struct {
	Bucket string
	Prefix string
}

type MultipartStager interface {
	StartMultipartUpload(ctx context.Context, input StartMultipartUploadInput) (StartMultipartUploadOutput, error)
	PresignUploadPart(ctx context.Context, input PresignUploadPartInput) (PresignUploadPartOutput, error)
	ListMultipartUploadParts(ctx context.Context, input ListMultipartUploadPartsInput) ([]UploadedPart, error)
	CompleteMultipartUpload(ctx context.Context, input CompleteMultipartUploadInput) error
	AbortMultipartUpload(ctx context.Context, input AbortMultipartUploadInput) error
	Stat(ctx context.Context, input StatInput) (ObjectStat, error)
}

type Downloader interface {
	Download(ctx context.Context, input DownloadInput) error
}

type Reader interface {
	OpenRead(ctx context.Context, input OpenReadInput) (OpenReadOutput, error)
}

type RangeReader interface {
	OpenReadRange(ctx context.Context, input OpenReadRangeInput) (OpenReadOutput, error)
}

type Uploader interface {
	Upload(ctx context.Context, input UploadInput) error
}

type Remover interface {
	Delete(ctx context.Context, input DeleteInput) error
}

type PrefixRemover interface {
	DeletePrefix(ctx context.Context, input DeletePrefixInput) error
}

type StreamingClient interface {
	MultipartStager
	Reader
	Uploader
	Remover
	PrefixRemover
}

type Client interface {
	MultipartStager
	Downloader
	Uploader
	Remover
}
