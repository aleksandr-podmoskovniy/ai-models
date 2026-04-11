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

package s3

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func (a *Adapter) Stat(
	ctx context.Context,
	input uploadstagingports.StatInput,
) (uploadstagingports.ObjectStat, error) {
	if err := validateStatInput(input); err != nil {
		return uploadstagingports.ObjectStat{}, err
	}
	output, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	})
	if err != nil {
		return uploadstagingports.ObjectStat{}, err
	}
	return uploadstagingports.ObjectStat{
		SizeBytes: aws.ToInt64(output.ContentLength),
		ETag:      strings.TrimSpace(aws.ToString(output.ETag)),
	}, nil
}

func (a *Adapter) Upload(ctx context.Context, input uploadstagingports.UploadInput) error {
	if err := validateUploadInput(input); err != nil {
		return err
	}

	_, err := a.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(strings.TrimSpace(input.Bucket)),
		Key:         aws.String(strings.TrimSpace(input.Key)),
		Body:        input.Body,
		ContentType: aws.String(strings.TrimSpace(input.ContentType)),
	})
	return err
}

func (a *Adapter) Download(ctx context.Context, input uploadstagingports.DownloadInput) error {
	if err := validateDownloadInput(input); err != nil {
		return err
	}

	file, err := os.OpenFile(input.DestinationPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = a.downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	})
	return err
}

func (a *Adapter) Delete(ctx context.Context, input uploadstagingports.DeleteInput) error {
	if err := validateDeleteInput(input); err != nil {
		return err
	}
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	})
	return err
}

func validateStatInput(input uploadstagingports.StatInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validateDownloadInput(input uploadstagingports.DownloadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.DestinationPath) == "":
		return errors.New("upload staging destination path must not be empty")
	default:
		return nil
	}
}

func validateUploadInput(input uploadstagingports.UploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case input.Body == nil:
		return errors.New("upload staging body must not be nil")
	default:
		return nil
	}
}

func validateDeleteInput(input uploadstagingports.DeleteInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}
