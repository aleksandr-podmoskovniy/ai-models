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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
)

type Config struct {
	EndpointURL     string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	UsePathStyle    bool
	Insecure        bool
	CAFile          string
}

type Adapter struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
}

func New(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	httpClient, err := newHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	awsConfig, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(strings.TrimSpace(cfg.Region)),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)),
		config.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		options.BaseEndpoint = aws.String(strings.TrimSpace(cfg.EndpointURL))
	})

	return &Adapter{
		client:     client,
		uploader:   manager.NewUploader(client),
		downloader: manager.NewDownloader(client),
	}, nil
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.EndpointURL) == "":
		return errors.New("upload staging s3 endpoint URL must not be empty")
	case strings.TrimSpace(c.Region) == "":
		return errors.New("upload staging s3 region must not be empty")
	case strings.TrimSpace(c.AccessKeyID) == "":
		return errors.New("upload staging s3 access key id must not be empty")
	case strings.TrimSpace(c.SecretAccessKey) == "":
		return errors.New("upload staging s3 secret access key must not be empty")
	default:
		return nil
	}
}

func (a *Adapter) Upload(ctx context.Context, input uploadstagingports.UploadInput) error {
	if err := validateUploadInput(input); err != nil {
		return err
	}
	_, err := a.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(strings.TrimSpace(input.Bucket)),
		Key:           aws.String(strings.TrimSpace(input.Key)),
		Body:          input.Body,
		ContentLength: aws.Int64(input.ContentLength),
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

func validateUploadInput(input uploadstagingports.UploadInput) error {
	switch {
	case strings.TrimSpace(input.Bucket) == "":
		return errors.New("upload staging bucket must not be empty")
	case strings.TrimSpace(input.Key) == "":
		return errors.New("upload staging key must not be empty")
	case input.Body == nil:
		return errors.New("upload staging body must not be nil")
	case input.ContentLength <= 0:
		return errors.New("upload staging content length must be positive")
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

func newHTTPClient(cfg Config) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.Insecure,
	}

	if !cfg.Insecure && strings.TrimSpace(cfg.CAFile) != "" {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system cert pool: %w", err)
		}
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		caData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, err
		}
		if ok := rootCAs.AppendCertsFromPEM(caData); !ok {
			return nil, errors.New("append upload staging CA bundle")
		}
		tlsConfig.RootCAs = rootCAs
	}

	transport.TLSClientConfig = tlsConfig
	return &http.Client{Transport: transport}, nil
}
