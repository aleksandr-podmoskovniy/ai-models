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
	"encoding/base64"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func TestTrustedFullObjectSHA256DigestAcceptsFullObjectChecksum(t *testing.T) {
	t.Parallel()

	checksum := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xaa", sha256DigestBytes)))

	got := trustedFullObjectSHA256Digest(aws.String(checksum), types.ChecksumTypeFullObject)
	want := "sha256:" + strings.Repeat("aa", sha256DigestBytes)
	if got != want {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want %q", got, want)
	}
}

func TestTrustedFullObjectSHA256DigestRejectsCompositeChecksum(t *testing.T) {
	t.Parallel()

	checksum := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("\xbb", sha256DigestBytes)))

	got := trustedFullObjectSHA256Digest(aws.String(checksum), types.ChecksumTypeComposite)
	if got != "" {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want empty digest", got)
	}
}

func TestTrustedFullObjectSHA256DigestRejectsMalformedChecksum(t *testing.T) {
	t.Parallel()

	got := trustedFullObjectSHA256Digest(aws.String("not-base64"), types.ChecksumTypeFullObject)
	if got != "" {
		t.Fatalf("trustedFullObjectSHA256Digest() = %q, want empty digest", got)
	}
}

func TestTrustedBackendVerificationReportsMissingChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes: 128,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumMissing {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumMissing)
	}
}

func TestTrustedBackendVerificationReportsCompositeChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes:             128,
		ReportedChecksumType:  "COMPOSITE",
		SHA256ChecksumPresent: true,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumComposite {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumComposite)
	}
}

func TestTrustedBackendVerificationReportsMalformedChecksumFallback(t *testing.T) {
	t.Parallel()

	_, ok, reason, err := trustedBackendVerification(ObjectAttributes{
		SizeBytes:             128,
		ReportedChecksumType:  checksumTypeFullObject,
		SHA256ChecksumPresent: true,
	})
	if err != nil {
		t.Fatalf("trustedBackendVerification() error = %v", err)
	}
	if ok {
		t.Fatal("trustedBackendVerification() ok = true, want false")
	}
	if reason != verificationFallbackReasonChecksumMalformed {
		t.Fatalf("trustedBackendVerification() reason = %q, want %q", reason, verificationFallbackReasonChecksumMalformed)
	}
}

func TestParseVerificationPolicyDefaultsToClientAsserted(t *testing.T) {
	t.Parallel()

	got, err := ParseVerificationPolicy("")
	if err != nil {
		t.Fatalf("ParseVerificationPolicy() error = %v", err)
	}
	if got != VerificationPolicyTrustedBackendOrClientAsserted {
		t.Fatalf("ParseVerificationPolicy() = %q, want %q", got, VerificationPolicyTrustedBackendOrClientAsserted)
	}
}

func TestParseVerificationPolicyRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	if _, err := ParseVerificationPolicy("unknown"); err == nil {
		t.Fatal("ParseVerificationPolicy() error = nil, want non-nil")
	}
}

func TestVerificationReadProgressWriterLogsOnByteThreshold(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 0).UTC()
	now := startedAt
	messages := make([]string, 0, 2)

	writer := newVerificationReadProgressWriter("dmcr/_ai_models/direct-upload/objects/session/data", 2<<30, startedAt)
	writer.now = func() time.Time { return now }
	writer.emit = func(format string, args ...any) {
		messages = append(messages, format)
	}

	if _, err := writer.Write(bytes.Repeat([]byte("a"), int(verificationReadProgressStepBytes))); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("log count = %d, want 1", len(messages))
	}
	if writer.nextProgressBytes != verificationReadProgressStepBytes*2 {
		t.Fatalf("nextProgressBytes = %d, want %d", writer.nextProgressBytes, verificationReadProgressStepBytes*2)
	}
}

func TestVerificationReadProgressWriterLogsOnTimeInterval(t *testing.T) {
	t.Parallel()

	startedAt := time.Unix(1_700_000_000, 0).UTC()
	now := startedAt
	messages := make([]string, 0, 2)

	writer := newVerificationReadProgressWriter("dmcr/_ai_models/direct-upload/objects/session/data", 768<<20, startedAt)
	writer.now = func() time.Time { return now }
	writer.emit = func(format string, args ...any) {
		messages = append(messages, format)
	}

	if _, err := writer.Write(bytes.Repeat([]byte("b"), 8<<20)); err != nil {
		t.Fatalf("first Write() error = %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("log count after first write = %d, want 0", len(messages))
	}

	now = startedAt.Add(verificationReadProgressInterval)
	if _, err := writer.Write(bytes.Repeat([]byte("c"), 8<<20)); err != nil {
		t.Fatalf("second Write() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("log count after second write = %d, want 1", len(messages))
	}
}

func TestVerificationLogsStructuredFields(t *testing.T) {
	var buffer bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buffer, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logClientAssertedVerification("dmcr/_ai_models/direct-upload/objects/session/data", verificationResult{
		Policy:                   VerificationPolicyTrustedBackendOrClientAsserted,
		Source:                   verificationSourceClientAsserted,
		FallbackReason:           verificationFallbackReasonChecksumMissing,
		BackendAttributesPresent: true,
		BackendSizeBytes:         42,
		BackendChecksumType:      "COMPOSITE",
		AvailableChecksums:       []string{"SHA256"},
		Sealed: sealedUpload{
			Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SizeBytes: 42,
		},
	}, sealedUpload{SizeBytes: 42}, nil)

	logs := buffer.String()
	if !strings.Contains(logs, `"msg":"direct upload verification accepted client-declared digest"`) {
		t.Fatalf("expected structured JSON log, got %q", logs)
	}
	if !strings.Contains(logs, `"objectKey":"dmcr/_ai_models/direct-upload/objects/session/data"`) {
		t.Fatalf("expected objectKey field, got %q", logs)
	}
	if strings.Contains(logs, "objectKey=") || strings.Contains(logs, "durationMs=") {
		t.Fatalf("log must not use printf-style key-value payloads: %q", logs)
	}
}
