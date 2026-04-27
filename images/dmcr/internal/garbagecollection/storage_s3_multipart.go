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
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

var errMultipartUploadGone = errors.New("multipart upload no longer exists")

func (s *s3PrefixStore) ForEachMultipartUpload(ctx context.Context, prefix string, visit func(multipartUploadInfo)) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	return s.forEachMultipartUploadPage(ctx, traversalListPrefix(prefix), func(uploads []types.MultipartUpload) error {
		for _, upload := range uploads {
			key := cleanStoragePath(aws.ToString(upload.Key))
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
		return nil
	})
}

func (s *s3PrefixStore) CountMultipartUploadParts(ctx context.Context, objectKey, uploadID string) (int, error) {
	if s == nil {
		return 0, errors.New("prefix store must not be nil")
	}

	cleanObjectKey, cleanUploadID, err := cleanMultipartUploadTarget(objectKey, uploadID)
	if err != nil {
		return 0, err
	}

	partCount := 0
	if err := s.forEachMultipartPartPage(ctx, cleanObjectKey, cleanUploadID, func(parts []types.Part) error {
		partCount += len(parts)
		return nil
	}); err != nil {
		return 0, err
	}
	return partCount, nil
}

func (s *s3PrefixStore) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	cleanObjectKey, cleanUploadID, err := cleanMultipartUploadTarget(objectKey, uploadID)
	if err != nil {
		return err
	}

	_, err = s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(cleanObjectKey),
		UploadId: aws.String(cleanUploadID),
	})
	if err != nil && !isNoSuchUploadError(err) {
		return err
	}
	return nil
}

func (s *s3PrefixStore) forEachMultipartUploadPage(ctx context.Context, listPrefix string, visit func([]types.MultipartUpload) error) error {
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
		if err := visit(output.Uploads); err != nil {
			return err
		}
		if !aws.ToBool(output.IsTruncated) {
			return nil
		}
		if err := requireNextMultipartUploadCursor(keyMarker, uploadIDMarker, output.NextKeyMarker, output.NextUploadIdMarker); err != nil {
			return err
		}
		keyMarker = output.NextKeyMarker
		uploadIDMarker = output.NextUploadIdMarker
	}
}

func (s *s3PrefixStore) forEachMultipartPartPage(ctx context.Context, objectKey, uploadID string, visit func([]types.Part) error) error {
	var partNumberMarker *string
	for {
		output, err := s.client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(s.bucket),
			Key:              aws.String(objectKey),
			UploadId:         aws.String(uploadID),
			PartNumberMarker: partNumberMarker,
		})
		if err != nil {
			if isNoSuchUploadError(err) {
				return errMultipartUploadGone
			}
			return err
		}
		if err := visit(output.Parts); err != nil {
			return err
		}
		if !aws.ToBool(output.IsTruncated) {
			return nil
		}
		if err := requireNextStringCursor("list multipart upload parts", partNumberMarker, output.NextPartNumberMarker); err != nil {
			return err
		}
		partNumberMarker = output.NextPartNumberMarker
	}
}

func requireNextMultipartUploadCursor(currentKey, currentUploadID, nextKey, nextUploadID *string) error {
	nextKeyValue := strings.TrimSpace(aws.ToString(nextKey))
	if nextKeyValue == "" {
		return errors.New("list multipart uploads returned truncated page without next cursor")
	}
	nextUploadIDValue := strings.TrimSpace(aws.ToString(nextUploadID))
	currentKeyValue := strings.TrimSpace(aws.ToString(currentKey))
	currentUploadIDValue := strings.TrimSpace(aws.ToString(currentUploadID))
	if nextKeyValue == currentKeyValue && nextUploadIDValue == "" {
		return fmt.Errorf("list multipart uploads returned truncated page without upload cursor for key %q", nextKeyValue)
	}
	if nextKeyValue == currentKeyValue && nextUploadIDValue == currentUploadIDValue {
		return fmt.Errorf("list multipart uploads returned truncated page with repeated cursor %q/%q", nextKeyValue, nextUploadIDValue)
	}
	return nil
}

func cleanMultipartUploadTarget(objectKey, uploadID string) (string, string, error) {
	cleanObjectKey := cleanStoragePath(objectKey)
	cleanUploadID := strings.TrimSpace(uploadID)
	if cleanObjectKey == "" {
		return "", "", errors.New("multipart object key must not be empty")
	}
	if cleanUploadID == "" {
		return "", "", errors.New("multipart upload ID must not be empty")
	}
	return cleanObjectKey, cleanUploadID, nil
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
	return fmt.Errorf("multipart upload %s (%s): %w", cleanStoragePath(objectKey), strings.TrimSpace(uploadID), err)
}
