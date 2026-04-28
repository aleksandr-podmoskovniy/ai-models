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
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3BackendConfig struct {
	Bucket        string
	Region        string
	EndpointURL   string
	UsePathStyle  bool
	Insecure      bool
	AccessKey     string
	SecretKey     string
	PresignExpiry time.Duration
}

type s3Backend struct {
	bucket  string
	client  *s3.Client
	presign *s3.PresignClient
}

func NewS3Backend(config S3BackendConfig) (Backend, error) {
	switch {
	case strings.TrimSpace(config.Bucket) == "":
		return nil, errors.New("direct upload S3 bucket must not be empty")
	case strings.TrimSpace(config.Region) == "":
		return nil, errors.New("direct upload S3 region must not be empty")
	case strings.TrimSpace(config.EndpointURL) == "":
		return nil, errors.New("direct upload S3 endpoint URL must not be empty")
	case strings.TrimSpace(config.AccessKey) == "":
		return nil, errors.New("direct upload S3 access key must not be empty")
	case strings.TrimSpace(config.SecretKey) == "":
		return nil, errors.New("direct upload S3 secret key must not be empty")
	}
	cfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(strings.TrimSpace(config.Region)),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(config.AccessKey),
			strings.TrimSpace(config.SecretKey),
			"",
		)),
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	)
	if err != nil {
		return nil, err
	}
	baseEndpoint := strings.TrimSpace(config.EndpointURL)
	if config.Insecure && baseEndpoint != "" {
		parsedEndpoint, err := url.Parse(baseEndpoint)
		if err != nil {
			return nil, err
		}
		parsedEndpoint.Scheme = "http"
		baseEndpoint = parsedEndpoint.String()
	}
	client := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.UsePathStyle = config.UsePathStyle
		options.BaseEndpoint = aws.String(baseEndpoint)
	})
	presignExpiry := config.PresignExpiry
	if presignExpiry <= 0 {
		presignExpiry = 15 * time.Minute
	}
	return &s3Backend{
		bucket: strings.TrimSpace(config.Bucket),
		client: client,
		presign: s3.NewPresignClient(client, func(options *s3.PresignOptions) {
			options.Expires = presignExpiry
		}),
	}, nil
}

func (b *s3Backend) ObjectExists(ctx context.Context, objectKey string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(strings.TrimSpace(objectKey)),
	})
	if err == nil {
		return true, nil
	}
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return false, nil
	}
	return false, err
}

func (b *s3Backend) ObjectAttributes(ctx context.Context, objectKey string) (ObjectAttributes, error) {
	output, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(b.bucket),
		Key:          aws.String(strings.TrimSpace(objectKey)),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	if err != nil {
		return ObjectAttributes{}, err
	}
	return ObjectAttributes{
		SizeBytes:                     aws.ToInt64(output.ContentLength),
		TrustedFullObjectSHA256Digest: trustedFullObjectSHA256Digest(output.ChecksumSHA256, output.ChecksumType),
		ReportedChecksumType:          strings.TrimSpace(string(output.ChecksumType)),
		SHA256ChecksumPresent:         strings.TrimSpace(aws.ToString(output.ChecksumSHA256)) != "",
		AvailableChecksumAlgorithms:   availableChecksumAlgorithms(output),
	}, nil
}

func availableChecksumAlgorithms(output *s3.HeadObjectOutput) []string {
	if output == nil {
		return nil
	}
	algorithms := make([]string, 0, 5)
	if strings.TrimSpace(aws.ToString(output.ChecksumCRC32)) != "" {
		algorithms = append(algorithms, "CRC32")
	}
	if strings.TrimSpace(aws.ToString(output.ChecksumCRC32C)) != "" {
		algorithms = append(algorithms, "CRC32C")
	}
	if strings.TrimSpace(aws.ToString(output.ChecksumCRC64NVME)) != "" {
		algorithms = append(algorithms, "CRC64NVME")
	}
	if strings.TrimSpace(aws.ToString(output.ChecksumSHA1)) != "" {
		algorithms = append(algorithms, "SHA1")
	}
	if strings.TrimSpace(aws.ToString(output.ChecksumSHA256)) != "" {
		algorithms = append(algorithms, "SHA256")
	}
	return algorithms
}

func trustedFullObjectSHA256Digest(checksum *string, checksumType types.ChecksumType) string {
	if checksum == nil || checksumType != types.ChecksumTypeFullObject {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(aws.ToString(checksum)))
	if err != nil || len(decoded) != sha256DigestBytes {
		return ""
	}
	return "sha256:" + hex.EncodeToString(decoded)
}

func (b *s3Backend) StartMultipartUpload(ctx context.Context, objectKey string) (string, error) {
	output, err := b.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(strings.TrimSpace(objectKey)),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(aws.ToString(output.UploadId)), nil
}

func (b *s3Backend) PresignUploadPart(ctx context.Context, objectKey, uploadID string, partNumber int) (string, error) {
	output, err := b.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(b.bucket),
		Key:        aws.String(strings.TrimSpace(objectKey)),
		UploadId:   aws.String(strings.TrimSpace(uploadID)),
		PartNumber: aws.Int32(int32(partNumber)),
	})
	if err != nil {
		return "", err
	}
	return output.URL, nil
}

func (b *s3Backend) ListUploadedParts(ctx context.Context, objectKey, uploadID string) ([]UploadedPart, error) {
	output, err := b.client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(b.bucket),
		Key:      aws.String(strings.TrimSpace(objectKey)),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
	})
	if err != nil {
		return nil, err
	}
	parts := make([]UploadedPart, 0, len(output.Parts))
	for _, part := range output.Parts {
		parts = append(parts, UploadedPart{
			PartNumber: int(aws.ToInt32(part.PartNumber)),
			ETag:       strings.Trim(strings.TrimSpace(aws.ToString(part.ETag)), "\""),
			SizeBytes:  aws.ToInt64(part.Size),
		})
	}
	return parts, nil
}

func (b *s3Backend) CompleteMultipartUpload(ctx context.Context, objectKey, uploadID string, parts []UploadedPart) error {
	completedParts := make([]types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		completedParts = append(completedParts, types.CompletedPart{
			ETag:       aws.String(strings.Trim(strings.TrimSpace(part.ETag), "\"")),
			PartNumber: aws.Int32(int32(part.PartNumber)),
		})
	}
	_, err := b.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(b.bucket),
		Key:      aws.String(strings.TrimSpace(objectKey)),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	return err
}

func (b *s3Backend) AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error {
	_, err := b.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(b.bucket),
		Key:      aws.String(strings.TrimSpace(objectKey)),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
	})
	return err
}

func (b *s3Backend) Reader(ctx context.Context, objectKey string, offset int64) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(strings.TrimSpace(objectKey)),
	}
	if offset > 0 {
		input.Range = aws.String("bytes=" + strconv.FormatInt(offset, 10) + "-")
	}
	output, err := b.client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (b *s3Backend) DeleteObject(ctx context.Context, objectKey string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(strings.TrimSpace(objectKey)),
	})
	return err
}

func (b *s3Backend) PutContent(ctx context.Context, objectKey string, payload []byte) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(strings.TrimSpace(objectKey)),
		Body:   bytes.NewReader(payload),
	})
	return err
}
