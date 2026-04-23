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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type prefixStore interface {
	ForEachObject(ctx context.Context, prefix string, visit func(string)) error
	ForEachObjectInfo(ctx context.Context, prefix string, visit func(prefixObjectInfo)) error
	ForEachMultipartUpload(ctx context.Context, prefix string, visit func(multipartUploadInfo)) error
	CountMultipartUploadParts(ctx context.Context, objectKey, uploadID string) (int, error)
	GetObject(ctx context.Context, key string) ([]byte, error)
	DeletePrefix(ctx context.Context, prefix string) error
	AbortMultipartUpload(ctx context.Context, objectKey, uploadID string) error
}

type prefixObjectInfo struct {
	Key          string
	LastModified time.Time
}

type multipartUploadInfo struct {
	Key         string
	UploadID    string
	InitiatedAt time.Time
}

type s3PrefixStore struct {
	bucket string
	client *s3.Client
}

func NewS3PrefixStore(config S3StorageConfig) (prefixStore, error) {
	switch {
	case strings.TrimSpace(config.Bucket) == "":
		return nil, errors.New("sealed S3 bucket must not be empty")
	case strings.TrimSpace(config.Region) == "":
		return nil, errors.New("sealed S3 region must not be empty")
	case strings.TrimSpace(config.EndpointURL) == "":
		return nil, errors.New("sealed S3 endpoint URL must not be empty")
	case strings.TrimSpace(config.AccessKey) == "":
		return nil, errors.New("sealed S3 access key must not be empty")
	case strings.TrimSpace(config.SecretKey) == "":
		return nil, errors.New("sealed S3 secret key must not be empty")
	}

	httpClient, err := newS3HTTPClient(config)
	if err != nil {
		return nil, err
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(strings.TrimSpace(config.Region)),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(config.AccessKey),
			strings.TrimSpace(config.SecretKey),
			"",
		)),
		awsconfig.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = config.UsePathStyle
		options.BaseEndpoint = aws.String(strings.TrimSpace(config.EndpointURL))
	})

	return &s3PrefixStore{
		bucket: strings.TrimSpace(config.Bucket),
		client: client,
	}, nil
}

func (s *s3PrefixStore) ForEachObject(ctx context.Context, prefix string, visit func(string)) error {
	return s.ForEachObjectInfo(ctx, prefix, func(info prefixObjectInfo) {
		visit(info.Key)
	})
}

func (s *s3PrefixStore) ForEachObjectInfo(ctx context.Context, prefix string, visit func(prefixObjectInfo)) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	listPrefix := normalizedListPrefix(prefix, false)
	var continuationToken *string
	for {
		output, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(listPrefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return err
		}

		for _, object := range output.Contents {
			key := strings.Trim(strings.TrimSpace(aws.ToString(object.Key)), "/")
			if key == "" {
				continue
			}
			visit(prefixObjectInfo{
				Key:          key,
				LastModified: aws.ToTime(object.LastModified).UTC(),
			})
		}

		if !aws.ToBool(output.IsTruncated) {
			return nil
		}
		continuationToken = output.NextContinuationToken
	}
}

func (s *s3PrefixStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("prefix store must not be nil")
	}

	cleanKey := strings.Trim(strings.TrimSpace(key), "/")
	if cleanKey == "" {
		return nil, errors.New("get object key must not be empty")
	}

	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()

	return io.ReadAll(output.Body)
}

func (s *s3PrefixStore) DeletePrefix(ctx context.Context, prefix string) error {
	if s == nil {
		return errors.New("prefix store must not be nil")
	}

	cleanPrefix := normalizedListPrefix(prefix, true)
	if strings.Trim(cleanPrefix, "/") == "" {
		return errors.New("delete prefix must not be empty")
	}

	var continuationToken *string
	for {
		listOutput, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(cleanPrefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return err
		}

		identifiers := make([]types.ObjectIdentifier, 0, len(listOutput.Contents))
		for _, object := range listOutput.Contents {
			key := strings.Trim(strings.TrimSpace(aws.ToString(object.Key)), "/")
			if key == "" {
				continue
			}
			identifiers = append(identifiers, types.ObjectIdentifier{Key: aws.String(key)})
		}

		if len(identifiers) > 0 {
			deleteOutput, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(s.bucket),
				Delete: &types.Delete{Objects: identifiers, Quiet: aws.Bool(true)},
			})
			if err != nil {
				return err
			}
			if err := deletePrefixErrors(deleteOutput.Errors); err != nil {
				return err
			}
		}

		if !aws.ToBool(listOutput.IsTruncated) {
			return nil
		}
		continuationToken = listOutput.NextContinuationToken
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

func normalizedListPrefix(prefix string, preserveTrailingSlash bool) string {
	cleanPrefix := strings.TrimLeft(strings.TrimSpace(prefix), "/")
	if !preserveTrailingSlash {
		cleanPrefix = strings.TrimRight(cleanPrefix, "/")
	}
	return cleanPrefix
}

func newS3HTTPClient(config S3StorageConfig) (*awshttp.BuildableClient, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: config.Insecure,
	}

	if !config.Insecure && strings.TrimSpace(config.CAFile) != "" {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system cert pool: %w", err)
		}
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}

		caData, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, err
		}
		if ok := rootCAs.AppendCertsFromPEM(caData); !ok {
			return nil, errors.New("append sealed S3 CA bundle")
		}
		tlsConfig.RootCAs = rootCAs
	}

	return awshttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
		transport.TLSClientConfig = tlsConfig
	}), nil
}
