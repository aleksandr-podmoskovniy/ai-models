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
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
)

type fakeBackend struct {
	objects          map[string][]byte
	attributes       map[string]ObjectAttributes
	uploads          map[string]string
	parts            map[string][]UploadedPart
	deleted          []string
	attributesCalls  int
	attributesErr    error
	readerCalls      int
	completeErr      error
	readerErr        error
	putErr           error
	putErrPathSuffix string
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		objects:    make(map[string][]byte),
		attributes: make(map[string]ObjectAttributes),
		uploads:    make(map[string]string),
		parts:      make(map[string][]UploadedPart),
	}
}

func (b *fakeBackend) ObjectExists(_ context.Context, objectKey string) (bool, error) {
	_, exists := b.objects[strings.TrimSpace(objectKey)]
	return exists, nil
}

func (b *fakeBackend) ObjectAttributes(_ context.Context, objectKey string) (ObjectAttributes, error) {
	b.attributesCalls++
	if b.attributesErr != nil {
		return ObjectAttributes{}, b.attributesErr
	}
	trimmed := strings.TrimSpace(objectKey)
	if attributes, exists := b.attributes[trimmed]; exists {
		return attributes, nil
	}
	payload, exists := b.objects[trimmed]
	if !exists {
		return ObjectAttributes{}, errors.New("object not found")
	}
	return ObjectAttributes{SizeBytes: int64(len(payload))}, nil
}

func (b *fakeBackend) StartMultipartUpload(_ context.Context, objectKey string) (string, error) {
	uploadID := "upload-" + objectKey
	b.uploads[uploadID] = objectKey
	return uploadID, nil
}

func (b *fakeBackend) PresignUploadPart(_ context.Context, _, uploadID string, partNumber int) (string, error) {
	return "https://upload.example/" + uploadID + "/" + strconv.Itoa(partNumber), nil
}

func (b *fakeBackend) ListUploadedParts(_ context.Context, objectKey, uploadID string) ([]UploadedPart, error) {
	_ = objectKey
	return append([]UploadedPart(nil), b.parts[uploadID]...), nil
}

func (b *fakeBackend) CompleteMultipartUpload(_ context.Context, objectKey, uploadID string, parts []UploadedPart) error {
	if b.completeErr != nil {
		return b.completeErr
	}
	b.parts[uploadID] = append([]UploadedPart(nil), parts...)
	b.objects[strings.TrimSpace(objectKey)] = payloadForParts(parts)
	return nil
}

func (b *fakeBackend) AbortMultipartUpload(_ context.Context, objectKey, uploadID string) error {
	delete(b.uploads, uploadID)
	delete(b.parts, uploadID)
	delete(b.objects, strings.TrimSpace(objectKey))
	return nil
}

func (b *fakeBackend) Reader(_ context.Context, objectKey string, offset int64) (io.ReadCloser, error) {
	b.readerCalls++
	if b.readerErr != nil {
		return nil, b.readerErr
	}
	payload, exists := b.objects[strings.TrimSpace(objectKey)]
	if !exists {
		return nil, errors.New("object not found")
	}
	if offset > int64(len(payload)) {
		offset = int64(len(payload))
	}
	return io.NopCloser(bytes.NewReader(payload[offset:])), nil
}

func (b *fakeBackend) DeleteObject(_ context.Context, objectKey string) error {
	trimmed := strings.TrimSpace(objectKey)
	delete(b.objects, trimmed)
	b.deleted = append(b.deleted, trimmed)
	return nil
}

func (b *fakeBackend) PutContent(_ context.Context, objectKey string, payload []byte) error {
	trimmed := strings.TrimSpace(objectKey)
	if b.putErr != nil && (b.putErrPathSuffix == "" || strings.HasSuffix(trimmed, b.putErrPathSuffix)) {
		return b.putErr
	}
	b.objects[trimmed] = append([]byte(nil), payload...)
	return nil
}
