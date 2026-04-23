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
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	httpClient *http.Client
	client     *s3.Client
	presign    *s3.PresignClient
	downloader *manager.Downloader
	uploader   *manager.Uploader
}

func New(cfg Config) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	awsHTTPClient, httpClient, err := newHTTPClients(cfg)
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
		config.WithHTTPClient(awsHTTPClient),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.UsePathStyle
		options.BaseEndpoint = aws.String(strings.TrimSpace(cfg.EndpointURL))
	})
	return &Adapter{
		httpClient: httpClient,
		client:     client,
		presign:    s3.NewPresignClient(client),
		downloader: manager.NewDownloader(client),
		uploader:   manager.NewUploader(client),
	}, nil
}

func (a *Adapter) HTTPClient() *http.Client {
	if a == nil {
		return nil
	}
	return a.httpClient
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

func newHTTPClients(cfg Config) (*awshttp.BuildableClient, *http.Client, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.Insecure,
	}

	if !cfg.Insecure && strings.TrimSpace(cfg.CAFile) != "" {
		rootCAs, err := x509.SystemCertPool()
		if err != nil {
			return nil, nil, fmt.Errorf("load system cert pool: %w", err)
		}
		if rootCAs == nil {
			rootCAs = x509.NewCertPool()
		}
		caData, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, nil, err
		}
		if ok := rootCAs.AppendCertsFromPEM(caData); !ok {
			return nil, nil, errors.New("append upload staging CA bundle")
		}
		tlsConfig.RootCAs = rootCAs
	}

	awsClient := awshttp.NewBuildableClient().WithTransportOptions(func(transport *http.Transport) {
		transport.TLSClientConfig = tlsConfig
	})
	return awsClient, &http.Client{Transport: awsClient.GetTransport().Clone()}, nil
}
