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
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCleanupPolicyForActiveRequestsTargetsDeleteTriggeredDirectUploadPrefix(t *testing.T) {
	t.Setenv(SealedS3AccessKeyEnv, "access")
	t.Setenv(SealedS3SecretKeyEnv, "secret")
	t.Setenv(directUploadTokenSecretEnv, "token-secret")

	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(`
storage:
  sealeds3:
    bucket: ai-models
    region: us-east-1
    regionendpoint: s3.example.com
    rootdirectory: dmcr
    forcepathstyle: true
    secure: true
    skipverify: false
`), 0o644); err != nil {
		t.Fatalf("os.WriteFile(config.yml) error = %v", err)
	}

	token := encodeDirectUploadSessionTokenForTest(t, []byte("token-secret"), directUploadSessionClaims{
		ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-a/data",
		UploadID:  "upload-a",
	})
	policy, err := cleanupPolicyForActiveRequests(configPath, []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dmcr-gc-request-a",
				Namespace: "d8-ai-models",
				Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
				Annotations: map[string]string{
					switchAnnotationKey:           "2026-04-23T13:00:00Z",
					directUploadModeAnnotationKey: directUploadModeImmediate,
					RequestQueuedAtAnnotationKey:  "2026-04-23T13:00:00Z",
				},
			},
			Data: map[string][]byte{
				directUploadTokenDataKey: []byte(token),
			},
		},
	})
	if err != nil {
		t.Fatalf("cleanupPolicyForActiveRequests() error = %v", err)
	}

	if _, found := policy.targetDirectUploadPrefixes["dmcr/_ai_models/direct-upload/objects/session-a"]; !found {
		t.Fatalf("expected targeted direct-upload prefix to be derived, got %#v", policy.targetDirectUploadPrefixes)
	}
	if _, found := policy.targetDirectUploadMultipartUploads[directUploadMultipartTarget{
		ObjectKey: "dmcr/_ai_models/direct-upload/objects/session-a/data",
		UploadID:  "upload-a",
	}]; !found {
		t.Fatalf("expected targeted direct-upload multipart upload to be derived, got %#v", policy.targetDirectUploadMultipartUploads)
	}
}

func TestCleanupPolicyForActiveRequestsIgnoresRequestsWithoutTargetedToken(t *testing.T) {
	t.Parallel()

	policy, err := cleanupPolicyForActiveRequests("unused", []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dmcr-gc-scheduled",
				Namespace: "d8-ai-models",
				Labels:    map[string]string{RequestLabelKey: RequestLabelValue},
				Annotations: map[string]string{
					switchAnnotationKey: "2026-04-23T13:00:00Z",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("cleanupPolicyForActiveRequests() error = %v", err)
	}
	if len(policy.targetDirectUploadPrefixes) != 0 {
		t.Fatalf("unexpected targeted direct-upload prefixes %#v", policy.targetDirectUploadPrefixes)
	}
	if len(policy.targetDirectUploadMultipartUploads) != 0 {
		t.Fatalf("unexpected targeted direct-upload multipart uploads %#v", policy.targetDirectUploadMultipartUploads)
	}
}

func encodeDirectUploadSessionTokenForTest(t *testing.T, secret []byte, claims directUploadSessionClaims) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	signature := directUploadSessionTokenSignature(secret, payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
}
