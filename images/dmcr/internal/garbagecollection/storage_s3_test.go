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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewS3PrefixStoreSupportsAWSCABundleEnv(t *testing.T) {
	caBundlePath := writeTestCABundle(t)
	t.Setenv(AWSCABundleEnv, caBundlePath)

	store, err := NewS3PrefixStore(S3StorageConfig{
		Bucket:       "k8s-mn-1-bucket",
		Region:       "rzn-pug",
		EndpointURL:  "https://s3.api.apiac.ru",
		UsePathStyle: true,
		AccessKey:    "test-access-key",
		SecretKey:    "test-secret-key",
	})
	if err != nil {
		t.Fatalf("NewS3PrefixStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil prefix store")
	}
}

func writeTestCABundle(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "ai-models-test-ca",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}

	caBundlePath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caBundlePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return caBundlePath
}
