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
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SealedS3AccessKeyEnv = "REGISTRY_STORAGE_SEALEDS3_ACCESSKEY"
	SealedS3SecretKeyEnv = "REGISTRY_STORAGE_SEALEDS3_SECRETKEY"
	AWSCABundleEnv       = "AWS_CA_BUNDLE"
)

type S3StorageConfig struct {
	Bucket        string
	Region        string
	EndpointURL   string
	RootDirectory string
	UsePathStyle  bool
	Insecure      bool
	AccessKey     string
	SecretKey     string
	CAFile        string
}

type registryConfig struct {
	Storage registryStorageConfig `yaml:"storage"`
}

type registryStorageConfig struct {
	SealedS3 registrySealedS3Config `yaml:"sealeds3"`
}

type registrySealedS3Config struct {
	Region         string `yaml:"region"`
	Bucket         string `yaml:"bucket"`
	RegionEndpoint string `yaml:"regionendpoint"`
	RootDirectory  string `yaml:"rootdirectory"`
	ForcePathStyle bool   `yaml:"forcepathstyle"`
	Secure         bool   `yaml:"secure"`
	SkipVerify     bool   `yaml:"skipverify"`
}

func LoadStorageConfig(configPath string) (S3StorageConfig, error) {
	config, err := loadRegistryConfig(configPath)
	if err != nil {
		return S3StorageConfig{}, err
	}

	endpointURL, err := buildEndpointURL(config.Storage.SealedS3.RegionEndpoint, config.Storage.SealedS3.Secure)
	if err != nil {
		return S3StorageConfig{}, err
	}

	accessKey := strings.TrimSpace(os.Getenv(SealedS3AccessKeyEnv))
	if accessKey == "" {
		return S3StorageConfig{}, fmt.Errorf("sealed S3 access key env %s must not be empty", SealedS3AccessKeyEnv)
	}
	secretKey := strings.TrimSpace(os.Getenv(SealedS3SecretKeyEnv))
	if secretKey == "" {
		return S3StorageConfig{}, fmt.Errorf("sealed S3 secret key env %s must not be empty", SealedS3SecretKeyEnv)
	}

	return S3StorageConfig{
		Bucket:        strings.TrimSpace(config.Storage.SealedS3.Bucket),
		Region:        strings.TrimSpace(config.Storage.SealedS3.Region),
		EndpointURL:   endpointURL,
		RootDirectory: strings.Trim(strings.TrimSpace(config.Storage.SealedS3.RootDirectory), "/"),
		UsePathStyle:  config.Storage.SealedS3.ForcePathStyle,
		Insecure:      config.Storage.SealedS3.SkipVerify,
		AccessKey:     accessKey,
		SecretKey:     secretKey,
		CAFile:        strings.TrimSpace(os.Getenv(AWSCABundleEnv)),
	}, nil
}

func loadRegistryConfig(configPath string) (registryConfig, error) {
	payload, err := os.ReadFile(configPath)
	if err != nil {
		return registryConfig{}, fmt.Errorf("read dmcr config: %w", err)
	}

	var config registryConfig
	if err := yaml.Unmarshal(payload, &config); err != nil {
		return registryConfig{}, fmt.Errorf("parse dmcr config: %w", err)
	}
	return config, nil
}

func buildEndpointURL(regionEndpoint string, secure bool) (string, error) {
	cleanEndpoint := strings.TrimSpace(regionEndpoint)
	if cleanEndpoint == "" {
		return "", fmt.Errorf("sealed S3 regionendpoint must not be empty")
	}
	if strings.Contains(cleanEndpoint, "://") {
		return cleanEndpoint, nil
	}
	scheme := "https"
	if !secure {
		scheme = "http"
	}
	return scheme + "://" + cleanEndpoint, nil
}
