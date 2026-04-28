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
		awsconfig.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		awsconfig.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
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

	return s.forEachObjectPage(ctx, traversalListPrefix(prefix), func(objects []types.Object) error {
		for _, object := range objects {
			key := cleanStoragePath(aws.ToString(object.Key))
			if key == "" {
				continue
			}
			visit(prefixObjectInfo{
				Key:          key,
				LastModified: aws.ToTime(object.LastModified).UTC(),
			})
		}
		return nil
	})
}

func (s *s3PrefixStore) GetObject(ctx context.Context, key string) ([]byte, error) {
	if s == nil {
		return nil, errors.New("prefix store must not be nil")
	}

	cleanKey := cleanStoragePath(key)
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

	cleanPrefix := destructiveListPrefix(prefix)
	if strings.Trim(cleanPrefix, "/") == "" {
		return errors.New("delete prefix must not be empty")
	}

	return s.forEachObjectPage(ctx, cleanPrefix, func(objects []types.Object) error {
		identifiers := make([]types.ObjectIdentifier, 0, len(objects))
		for _, object := range objects {
			key := cleanStoragePath(aws.ToString(object.Key))
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
		return nil
	})
}

func (s *s3PrefixStore) forEachObjectPage(ctx context.Context, listPrefix string, visit func([]types.Object) error) error {
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
		if err := visit(output.Contents); err != nil {
			return err
		}
		if !aws.ToBool(output.IsTruncated) {
			return nil
		}
		if err := requireNextStringCursor("list S3 objects", continuationToken, output.NextContinuationToken); err != nil {
			return err
		}
		continuationToken = output.NextContinuationToken
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

func traversalListPrefix(prefix string) string {
	cleanPrefix := strings.TrimLeft(strings.TrimSpace(prefix), "/")
	return strings.TrimRight(cleanPrefix, "/")
}

func destructiveListPrefix(prefix string) string {
	return strings.TrimLeft(strings.TrimSpace(prefix), "/")
}

func requireNextStringCursor(operation string, current, next *string) error {
	nextValue := strings.TrimSpace(aws.ToString(next))
	if nextValue == "" {
		return fmt.Errorf("%s returned truncated page without next cursor", operation)
	}
	currentValue := strings.TrimSpace(aws.ToString(current))
	if currentValue != "" && currentValue == nextValue {
		return fmt.Errorf("%s returned truncated page with repeated cursor %q", operation, nextValue)
	}
	return nil
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
