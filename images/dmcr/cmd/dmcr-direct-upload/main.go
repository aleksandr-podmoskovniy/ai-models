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

package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/directupload"
	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
)

const (
	listenAddressEnv      = "DMCR_DIRECT_UPLOAD_LISTEN_ADDRESS"
	tlsCertFileEnv        = "DMCR_DIRECT_UPLOAD_TLS_CERT_FILE"
	tlsKeyFileEnv         = "DMCR_DIRECT_UPLOAD_TLS_KEY_FILE"
	authUsernameEnv       = "DMCR_DIRECT_UPLOAD_USERNAME"
	authPasswordEnv       = "DMCR_DIRECT_UPLOAD_PASSWORD"
	tokenSecretEnv        = "REGISTRY_HTTP_SECRET"
	rootDirectoryEnv      = "DMCR_STORAGE_S3_ROOT_DIRECTORY"
	bucketEnv             = "DMCR_STORAGE_S3_BUCKET"
	regionEnv             = "DMCR_STORAGE_S3_REGION"
	endpointEnv           = "DMCR_STORAGE_S3_ENDPOINT"
	usePathStyleEnv       = "DMCR_STORAGE_S3_USE_PATH_STYLE"
	insecureEnv           = "DMCR_STORAGE_S3_INSECURE"
	accessKeyEnv          = "REGISTRY_STORAGE_S3_ACCESSKEY"
	secretKeyEnv          = "REGISTRY_STORAGE_S3_SECRETKEY"
	partSizeBytesEnv      = "DMCR_DIRECT_UPLOAD_PART_SIZE_BYTES"
	presignExpiryEnv      = "DMCR_DIRECT_UPLOAD_PRESIGN_EXPIRY"
	sessionTTLEnv         = "DMCR_DIRECT_UPLOAD_SESSION_TTL"
	verificationPolicyEnv = "DMCR_DIRECT_UPLOAD_VERIFICATION_POLICY"
	defaultListenAddr     = ":5002"
	defaultPresignLife    = 15 * time.Minute
)

func main() {
	backend, err := directupload.NewS3Backend(directupload.S3BackendConfig{
		Bucket:        env(bucketEnv),
		Region:        env(regionEnv),
		EndpointURL:   env(endpointEnv),
		UsePathStyle:  envBool(usePathStyleEnv),
		Insecure:      envBool(insecureEnv),
		AccessKey:     env(accessKeyEnv),
		SecretKey:     env(secretKeyEnv),
		PresignExpiry: envDuration(presignExpiryEnv, defaultPresignLife),
	})
	if err != nil {
		log.Fatal(err)
	}

	service, err := directupload.NewService(
		backend,
		env(authUsernameEnv),
		env(authPasswordEnv),
		env(tokenSecretEnv),
		env(rootDirectoryEnv),
		envInt64(partSizeBytesEnv, directupload.DefaultBlobPartSizeBytes),
		envDuration(sessionTTLEnv, directupload.DefaultSessionTTL),
	)
	if err != nil {
		log.Fatal(err)
	}
	verificationPolicy, err := directupload.ParseVerificationPolicy(env(verificationPolicyEnv))
	if err != nil {
		log.Fatal(err)
	}
	if err := service.SetVerificationPolicy(verificationPolicy); err != nil {
		log.Fatal(err)
	}
	maintenanceChecker, err := maintenance.NewFileCheckerFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	service.SetMaintenanceChecker(maintenanceChecker)
	observer, err := maintenance.NewFileAckObserverFromEnv("direct-upload", 0)
	if err != nil {
		log.Fatal(err)
	}
	if observer != nil {
		observer.Start(context.Background())
	}

	server, err := directupload.NewServer(
		envDefault(listenAddressEnv, defaultListenAddr),
		env(tlsCertFileEnv),
		env(tlsKeyFileEnv),
		service,
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(server.ListenAndServeTLS(env(tlsCertFileEnv), env(tlsKeyFileEnv)))
}

func env(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func envDefault(name, fallback string) string {
	if value := env(name); value != "" {
		return value
	}
	return fallback
}

func envBool(name string) bool {
	value := strings.ToLower(env(name))
	return value == "true" || value == "1" || value == "yes"
}

func envInt64(name string, fallback int64) int64 {
	value := env(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := env(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
