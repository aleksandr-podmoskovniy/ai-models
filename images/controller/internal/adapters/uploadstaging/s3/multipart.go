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
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

func (a *Adapter) StartMultipartUpload(
	ctx context.Context,
	input uploadstagingports.StartMultipartUploadInput,
) (uploadstagingports.StartMultipartUploadOutput, error) {
	if err := validateStartMultipartUploadInput(input); err != nil {
		return uploadstagingports.StartMultipartUploadOutput{}, err
	}
	output, err := a.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(strings.TrimSpace(input.Bucket)),
		Key:    aws.String(strings.TrimSpace(input.Key)),
	})
	if err != nil {
		return uploadstagingports.StartMultipartUploadOutput{}, err
	}
	return uploadstagingports.StartMultipartUploadOutput{
		UploadID: strings.TrimSpace(aws.ToString(output.UploadId)),
	}, nil
}

func (a *Adapter) PresignUploadPart(
	ctx context.Context,
	input uploadstagingports.PresignUploadPartInput,
) (uploadstagingports.PresignUploadPartOutput, error) {
	if err := validatePresignUploadPartInput(input); err != nil {
		return uploadstagingports.PresignUploadPartOutput{}, err
	}
	output, err := a.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(strings.TrimSpace(input.Bucket)),
		Key:        aws.String(strings.TrimSpace(input.Key)),
		UploadId:   aws.String(strings.TrimSpace(input.UploadID)),
		PartNumber: aws.Int32(input.PartNumber),
	}, func(options *s3.PresignOptions) {
		options.Expires = input.Expires
	})
	if err != nil {
		return uploadstagingports.PresignUploadPartOutput{}, err
	}
	return uploadstagingports.PresignUploadPartOutput{URL: output.URL}, nil
}

func (a *Adapter) ListMultipartUploadParts(
	ctx context.Context,
	input uploadstagingports.ListMultipartUploadPartsInput,
) ([]uploadstagingports.UploadedPart, error) {
	if err := validateListMultipartUploadPartsInput(input); err != nil {
		return nil, err
	}

	result := make([]uploadstagingports.UploadedPart, 0)
	var marker *string
	for {
		output, err := a.client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(strings.TrimSpace(input.Bucket)),
			Key:              aws.String(strings.TrimSpace(input.Key)),
			UploadId:         aws.String(strings.TrimSpace(input.UploadID)),
			PartNumberMarker: marker,
		})
		if err != nil {
			return nil, err
		}
		for _, part := range output.Parts {
			result = append(result, uploadstagingports.UploadedPart{
				PartNumber: aws.ToInt32(part.PartNumber),
				ETag:       strings.TrimSpace(aws.ToString(part.ETag)),
				SizeBytes:  aws.ToInt64(part.Size),
			})
		}
		if !aws.ToBool(output.IsTruncated) {
			break
		}
		marker = output.NextPartNumberMarker
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PartNumber < result[j].PartNumber
	})
	return result, nil
}

func (a *Adapter) CompleteMultipartUpload(
	ctx context.Context,
	input uploadstagingports.CompleteMultipartUploadInput,
) error {
	if err := validateCompleteMultipartUploadInput(input); err != nil {
		return err
	}
	parts := make([]s3types.CompletedPart, 0, len(input.Parts))
	for _, part := range sortCompletedParts(input.Parts) {
		parts = append(parts, s3types.CompletedPart{
			ETag:       aws.String(strings.TrimSpace(part.ETag)),
			PartNumber: aws.Int32(part.PartNumber),
		})
	}
	_, err := a.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(strings.TrimSpace(input.Bucket)),
		Key:      aws.String(strings.TrimSpace(input.Key)),
		UploadId: aws.String(strings.TrimSpace(input.UploadID)),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	return err
}

func (a *Adapter) AbortMultipartUpload(
	ctx context.Context,
	input uploadstagingports.AbortMultipartUploadInput,
) error {
	if err := validateAbortMultipartUploadInput(input); err != nil {
		return err
	}
	_, err := a.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(strings.TrimSpace(input.Bucket)),
		Key:      aws.String(strings.TrimSpace(input.Key)),
		UploadId: aws.String(strings.TrimSpace(input.UploadID)),
	})
	return err
}

func validateStartMultipartUploadInput(input uploadstagingports.StartMultipartUploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	default:
		return nil
	}
}

func validatePresignUploadPartInput(input uploadstagingports.PresignUploadPartInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.UploadID) == "":
		return errors.New("upload staging upload id must not be empty")
	case input.PartNumber <= 0:
		return errors.New("upload staging part number must be positive")
	case input.Expires <= 0:
		return errors.New("upload staging presign expiry must be positive")
	default:
		return nil
	}
}

func validateCompleteMultipartUploadInput(input uploadstagingports.CompleteMultipartUploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.UploadID) == "":
		return errors.New("upload staging upload id must not be empty")
	case len(input.Parts) == 0:
		return errors.New("upload staging completed parts must not be empty")
	}
	for _, part := range input.Parts {
		switch {
		case part.PartNumber <= 0:
			return errors.New("upload staging completed part number must be positive")
		case strings.TrimSpace(part.ETag) == "":
			return errors.New("upload staging completed part ETag must not be empty")
		}
	}
	return nil
}

func validateListMultipartUploadPartsInput(input uploadstagingports.ListMultipartUploadPartsInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.UploadID) == "":
		return errors.New("upload staging upload id must not be empty")
	default:
		return nil
	}
}

func validateAbortMultipartUploadInput(input uploadstagingports.AbortMultipartUploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case strings.TrimSpace(input.UploadID) == "":
		return errors.New("upload staging upload id must not be empty")
	default:
		return nil
	}
}

func sortCompletedParts(parts []uploadstagingports.CompletedPart) []uploadstagingports.CompletedPart {
	sorted := append([]uploadstagingports.CompletedPart(nil), parts...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PartNumber < sorted[j].PartNumber
	})
	return sorted
}
