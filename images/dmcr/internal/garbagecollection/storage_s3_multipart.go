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

package garbagecollection

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

var errMultipartUploadGone = errors.New("multipart upload no longer exists")

func (s *s3PrefixStore) ForEachMultipartUpload(ctx context.Context, prefix string, visit func(multipartUploadInfo)) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	listPrefix := normalizedListPrefix(prefix, false)
	var (
		keyMarker      *string
		uploadIDMarker *string
	)
	for {
		output, err := s.client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
			Bucket:         aws.String(s.bucket),
			Prefix:         aws.String(listPrefix),
			KeyMarker:      keyMarker,
			UploadIdMarker: uploadIDMarker,
		})
		if err != nil {
			return err
		}

		for _, upload := range output.Uploads {
			key := strings.Trim(strings.TrimSpace(aws.ToString(upload.Key)), "/")
			uploadID := strings.TrimSpace(aws.ToString(upload.UploadId))
			if key == "" || uploadID == "" {
				continue
			}
			visit(multipartUploadInfo{
				Key:         key,
				UploadID:    uploadID,
				InitiatedAt: aws.ToTime(upload.Initiated).UTC(),
			})
		}

		if !aws.ToBool(output.IsTruncated) {
			return nil
		}
		keyMarker = output.NextKeyMarker
		uploadIDMarker = output.NextUploadIdMarker
	}
}

func (s *s3PrefixStore) CountMultipartUploadParts(ctx context.Context, objectKey, uploadID string) (int, error) {
	if s == nil {
		return 0, errors.New("prefix store must not be nil")
	}

	cleanObjectKey := strings.Trim(strings.TrimSpace(objectKey), "/")
	cleanUploadID := strings.TrimSpace(uploadID)
	if cleanObjectKey == "" {
		return 0, errors.New("multipart object key must not be empty")
	}
	if cleanUploadID == "" {
		return 0, errors.New("multipart upload ID must not be empty")
	}

	partCount := 0
	var partNumberMarker *string
	for {
		output, err := s.client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(s.bucket),
			Key:              aws.String(cleanObjectKey),
			UploadId:         aws.String(cleanUploadID),
			PartNumberMarker: partNumberMarker,
		})
		if err != nil {
			if isNoSuchUploadError(err) {
				return 0, errMultipartUploadGone
			}
			return 0, err
		}

		partCount += len(output.Parts)
		if !aws.ToBool(output.IsTruncated) {
			return partCount, nil
		}
		partNumberMarker = output.NextPartNumberMarker
	}
}

func (s *s3PrefixStore) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	cleanObjectKey := strings.Trim(strings.TrimSpace(objectKey), "/")
	cleanUploadID := strings.TrimSpace(uploadID)
	if cleanObjectKey == "" {
		return errors.New("multipart object key must not be empty")
	}
	if cleanUploadID == "" {
		return errors.New("multipart upload ID must not be empty")
	}

	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(cleanObjectKey),
		UploadId: aws.String(cleanUploadID),
	})
	if err != nil && !isNoSuchUploadError(err) {
		return err
	}
	return nil
}

func isNoSuchUploadError(err error) bool {
	if err == nil {
		return false
	}

	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		return strings.TrimSpace(apiError.ErrorCode()) == "NoSuchUpload"
	}
	return false
}

func formatMultipartUploadTargetError(objectKey, uploadID string, err error) error {
	return fmt.Errorf("multipart upload %s (%s): %w", strings.Trim(strings.TrimSpace(objectKey), "/"), strings.TrimSpace(uploadID), err)
}
