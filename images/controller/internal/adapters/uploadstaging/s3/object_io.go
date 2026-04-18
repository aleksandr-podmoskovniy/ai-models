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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
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

func (a *Adapter) OpenRead(
	ctx context.Context,
	input uploadstagingports.OpenReadInput,
) (uploadstagingports.OpenReadOutput, error) {
	if err := validateOpenReadInput(input); err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	output, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	})
	if err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	return uploadstagingports.OpenReadOutput{
		Body:      output.Body,
		SizeBytes: aws.ToInt64(output.ContentLength),
		ETag:      strings.TrimSpace(aws.ToString(output.ETag)),
	}, nil
}

func (a *Adapter) OpenReadRange(
	ctx context.Context,
	input uploadstagingports.OpenReadRangeInput,
) (uploadstagingports.OpenReadOutput, error) {
	if err := validateOpenReadRangeInput(input); err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	getInput := &s3.GetObjectInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	}
	if rangeHeader, ok := objectRangeHeader(input.Offset, input.Length); ok {
		getInput.Range = aws.String(rangeHeader)
	}
	output, err := a.client.GetObject(ctx, getInput)
	if err != nil {
		return uploadstagingports.OpenReadOutput{}, err
	}
	return uploadstagingports.OpenReadOutput{
		Body:      output.Body,
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

func (a *Adapter) DeletePrefix(ctx context.Context, input uploadstagingports.DeletePrefixInput) error {
	if err := validateDeletePrefixInput(input); err != nil {
		return err
	}

	bucket := strings.TrimSpace(input.Bucket)
	prefix := strings.TrimSpace(input.Prefix)
	var continuationToken *string

	for {
		listOutput, err := a.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return err
		}
		if len(listOutput.Contents) == 0 {
			if !aws.ToBool(listOutput.IsTruncated) {
				return nil
			}
			continuationToken = listOutput.NextContinuationToken
			continue
		}

		identifiers := make([]types.ObjectIdentifier, 0, len(listOutput.Contents))
		for _, object := range listOutput.Contents {
			key := strings.TrimSpace(aws.ToString(object.Key))
			if key == "" {
				continue
			}
			identifiers = append(identifiers, types.ObjectIdentifier{Key: aws.String(key)})
		}
		if len(identifiers) == 0 {
			if !aws.ToBool(listOutput.IsTruncated) {
				return nil
			}
			continuationToken = listOutput.NextContinuationToken
			continue
		}

		deleteOutput, err := a.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{Objects: identifiers, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return err
		}
		if err := deletePrefixErrors(deleteOutput.Errors); err != nil {
			return err
		}

		if !aws.ToBool(listOutput.IsTruncated) {
			return nil
		}
		continuationToken = listOutput.NextContinuationToken
	}
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

func validateOpenReadInput(input uploadstagingports.OpenReadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validateOpenReadRangeInput(input uploadstagingports.OpenReadRangeInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case input.Offset < 0:
		return errors.New("upload staging read offset must not be negative")
	case input.Length == 0:
		return errors.New("upload staging read length must not be zero")
	case input.Length < -1:
		return errors.New("upload staging read length must be positive or -1")
	default:
		return nil
	}
}

func objectRangeHeader(offset, length int64) (string, bool) {
	if offset <= 0 && length < 0 {
		return "", false
	}
	if length < 0 {
		return "bytes=" + strconv.FormatInt(offset, 10) + "-", true
	}
	return "bytes=" + strconv.FormatInt(offset, 10) + "-" + strconv.FormatInt(offset+length-1, 10), true
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

func validateDeletePrefixInput(input uploadstagingports.DeletePrefixInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Prefix) == "":
		return errors.New("upload staging prefix must not be empty")
	default:
		return nil
	}
}

func deletePrefixErrors(errors []types.Error) error {
	if len(errors) == 0 {
		return nil
	}

	messages := make([]string, 0, len(errors))
	for _, entry := range errors {
		key := strings.TrimSpace(aws.ToString(entry.Key))
		code := strings.TrimSpace(aws.ToString(entry.Code))
		message := strings.TrimSpace(aws.ToString(entry.Message))
		switch {
		case key != "" && code != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s (%s: %s)", key, code, message))
		case key != "" && code != "":
			messages = append(messages, fmt.Sprintf("%s (%s)", key, code))
		case key != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s (%s)", key, message))
		case key != "":
			messages = append(messages, key)
		case code != "" && message != "":
			messages = append(messages, fmt.Sprintf("%s: %s", code, message))
		case code != "":
			messages = append(messages, code)
		case message != "":
			messages = append(messages, message)
		default:
			messages = append(messages, "unknown deleteObjects error")
		}
	}

	return fmt.Errorf("delete prefix returned object errors: %s", strings.Join(messages, ", "))
}
