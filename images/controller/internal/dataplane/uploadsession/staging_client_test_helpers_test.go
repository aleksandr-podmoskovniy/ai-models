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

package uploadsession

import (
	"context"

	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type fakeStagingClient struct {
	started         uploadstagingports.StartMultipartUploadInput
	startOutput     uploadstagingports.StartMultipartUploadOutput
	startErr        error
	presignInputs   []uploadstagingports.PresignUploadPartInput
	presignedURL    string
	presignErr      error
	listPartsInput  uploadstagingports.ListMultipartUploadPartsInput
	listPartsOutput []uploadstagingports.UploadedPart
	listPartsErr    error
	completed       uploadstagingports.CompleteMultipartUploadInput
	completeErr     error
	aborted         uploadstagingports.AbortMultipartUploadInput
	abortErr        error
	statInput       uploadstagingports.StatInput
	statOutput      uploadstagingports.ObjectStat
	statErr         error
	deleted         uploadstagingports.DeleteInput
	deleteErr       error
}

func (c *fakeStagingClient) StartMultipartUpload(_ context.Context, input uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	c.started = input
	if c.startErr != nil {
		return uploadstagingports.StartMultipartUploadOutput{}, c.startErr
	}
	return c.startOutput, nil
}

func (c *fakeStagingClient) PresignUploadPart(_ context.Context, input uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	c.presignInputs = append(c.presignInputs, input)
	if c.presignErr != nil {
		return uploadstagingports.PresignUploadPartOutput{}, c.presignErr
	}
	return uploadstagingports.PresignUploadPartOutput{URL: c.presignedURL}, nil
}

func (c *fakeStagingClient) CompleteMultipartUpload(_ context.Context, input uploadstagingports.CompleteMultipartUploadInput) error {
	c.completed = input
	return c.completeErr
}

func (c *fakeStagingClient) ListMultipartUploadParts(_ context.Context, input uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	c.listPartsInput = input
	if c.listPartsErr != nil {
		return nil, c.listPartsErr
	}
	return append([]uploadstagingports.UploadedPart(nil), c.listPartsOutput...), nil
}

func (c *fakeStagingClient) AbortMultipartUpload(_ context.Context, input uploadstagingports.AbortMultipartUploadInput) error {
	c.aborted = input
	return c.abortErr
}

func (c *fakeStagingClient) Stat(_ context.Context, input uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	c.statInput = input
	if c.statErr != nil {
		return uploadstagingports.ObjectStat{}, c.statErr
	}
	return c.statOutput, nil
}

func (c *fakeStagingClient) Download(context.Context, uploadstagingports.DownloadInput) error {
	return nil
}

func (c *fakeStagingClient) Upload(context.Context, uploadstagingports.UploadInput) error {
	return nil
}

func (c *fakeStagingClient) Delete(_ context.Context, input uploadstagingports.DeleteInput) error {
	c.deleted = input
	return c.deleteErr
}
