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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"

	digest "github.com/opencontainers/go-digest"
)

const sha256DigestBytes = sha256.Size

const (
	verificationReadProgressStepBytes int64 = 1 << 30
	verificationReadProgressInterval        = 30 * time.Second
	checksumTypeFullObject                  = "FULL_OBJECT"
)

type verificationSource string

const (
	verificationSourceTrustedBackendSHA256 verificationSource = "trusted-backend-sha256"
	verificationSourceClientAsserted       verificationSource = "client-asserted"
	verificationSourceObjectRead           verificationSource = "object-reread"
)

type verificationFallbackReason string

const (
	verificationFallbackReasonNone              verificationFallbackReason = ""
	verificationFallbackReasonAttributesError   verificationFallbackReason = "attributes-error"
	verificationFallbackReasonChecksumMissing   verificationFallbackReason = "checksum-missing"
	verificationFallbackReasonChecksumComposite verificationFallbackReason = "checksum-composite"
	verificationFallbackReasonChecksumMalformed verificationFallbackReason = "checksum-malformed"
	verificationFallbackReasonDigestMissing     verificationFallbackReason = "digest-missing"
)

type verificationResult struct {
	Sealed                   sealedUpload
	Policy                   VerificationPolicy
	Source                   verificationSource
	FallbackReason           verificationFallbackReason
	BackendAttributesPresent bool
	BackendSizeBytes         int64
	BackendChecksumType      string
	BackendSHA256Present     bool
	AvailableChecksums       []string
}

func (s *Service) verifyUploadedObject(ctx context.Context, objectKey string, expected sealedUpload) (verificationResult, error) {
	objectKey = strings.TrimSpace(objectKey)
	expected.Digest = strings.TrimSpace(expected.Digest)
	result, err := s.inspectBackendVerification(ctx, objectKey)
	result.Policy = s.verificationPolicy
	if err != nil {
		result = verificationResult{
			Policy:         s.verificationPolicy,
			FallbackReason: verificationFallbackReasonAttributesError,
		}
		return s.resolveFallbackVerification(ctx, objectKey, expected, result, err)
	}
	if result.Source == verificationSourceTrustedBackendSHA256 {
		slog.Default().Info(
			"direct upload verification selected trusted backend checksum",
			slog.String("objectKey", objectKey),
			slog.String("verificationPolicy", string(result.Policy)),
			slog.String("verificationSource", string(result.Source)),
			slog.String("artifactDigest", result.Sealed.Digest),
			slog.Int64("sizeBytes", result.Sealed.SizeBytes),
			slog.String("backendChecksumType", result.BackendChecksumType),
			slog.Bool("backendSHA256Present", result.BackendSHA256Present),
			slog.String("availableChecksums", strings.Join(result.AvailableChecksums, ",")),
		)
		return result, nil
	}

	return s.resolveFallbackVerification(ctx, objectKey, expected, result, nil)
}

func (s *Service) inspectBackendVerification(ctx context.Context, objectKey string) (verificationResult, error) {
	attributes, err := s.backend.ObjectAttributes(ctx, strings.TrimSpace(objectKey))
	if err != nil {
		return verificationResult{}, err
	}

	result := verificationResult{
		BackendAttributesPresent: true,
		BackendSizeBytes:         attributes.SizeBytes,
		BackendChecksumType:      strings.ToUpper(strings.TrimSpace(attributes.ReportedChecksumType)),
		BackendSHA256Present:     attributes.SHA256ChecksumPresent,
		AvailableChecksums:       slices.Clone(attributes.AvailableChecksumAlgorithms),
	}
	sealed, ok, reason, err := trustedBackendVerification(attributes)
	if err != nil {
		return verificationResult{}, err
	}
	if ok {
		result.Source = verificationSourceTrustedBackendSHA256
		result.Sealed = sealed
		return result, nil
	}
	result.FallbackReason = reason
	return result, nil
}

func (s *Service) resolveFallbackVerification(
	ctx context.Context,
	objectKey string,
	expected sealedUpload,
	result verificationResult,
	attributesErr error,
) (verificationResult, error) {
	if expected.Digest != "" && !s.verificationPolicy.requiresObjectRereadFallback() {
		result.Source = verificationSourceClientAsserted
		result.Sealed = sealedUpload{
			Digest:    expected.Digest,
			SizeBytes: result.resolvedSizeBytes(expected.SizeBytes),
		}
		logClientAssertedVerification(objectKey, result, expected, attributesErr)
		return result, nil
	}

	if expected.Digest == "" {
		result.FallbackReason = verificationFallbackReasonDigestMissing
	}
	slog.Default().Info(
		"direct upload verification falling back to object reread",
		slog.String("objectKey", objectKey),
		slog.String("verificationPolicy", string(result.Policy)),
		slog.String("verificationSource", string(verificationSourceObjectRead)),
		slog.String("fallbackReason", string(result.FallbackReason)),
		slog.Bool("backendAttributesPresent", result.BackendAttributesPresent),
		slog.Int64("backendSizeBytes", result.BackendSizeBytes),
		slog.String("backendChecksumType", result.BackendChecksumType),
		slog.Bool("backendSHA256Present", result.BackendSHA256Present),
		slog.String("availableChecksums", strings.Join(result.AvailableChecksums, ",")),
	)
	return s.verifyUploadedObjectByReading(ctx, objectKey, expected.SizeBytes, result)
}

func logClientAssertedVerification(objectKey string, result verificationResult, expected sealedUpload, attributesErr error) {
	args := []any{
		slog.String("objectKey", objectKey),
		slog.String("verificationPolicy", string(result.Policy)),
		slog.String("verificationSource", string(result.Source)),
		slog.String("fallbackReason", string(result.FallbackReason)),
		slog.String("declaredDigest", result.Sealed.Digest),
		slog.Int64("declaredSizeBytes", expected.SizeBytes),
		slog.Bool("backendAttributesPresent", result.BackendAttributesPresent),
		slog.Int64("backendSizeBytes", result.BackendSizeBytes),
		slog.String("backendChecksumType", result.BackendChecksumType),
		slog.Bool("backendSHA256Present", result.BackendSHA256Present),
		slog.String("availableChecksums", strings.Join(result.AvailableChecksums, ",")),
	}
	if attributesErr != nil {
		args = append(args, slog.Any("error", attributesErr))
	}
	slog.Default().Info("direct upload verification accepted client-declared digest", args...)
}

func (r verificationResult) resolvedSizeBytes(fallback int64) int64 {
	if r.BackendAttributesPresent && r.BackendSizeBytes > 0 {
		return r.BackendSizeBytes
	}
	return fallback
}

func trustedBackendVerification(attributes ObjectAttributes) (sealedUpload, bool, verificationFallbackReason, error) {
	if attributes.SizeBytes < 0 {
		return sealedUpload{}, false, verificationFallbackReasonNone, fmt.Errorf("trusted backend sizeBytes must not be negative")
	}
	if !attributes.SHA256ChecksumPresent {
		return sealedUpload{}, false, verificationFallbackReasonChecksumMissing, nil
	}
	if strings.ToUpper(strings.TrimSpace(attributes.ReportedChecksumType)) != checksumTypeFullObject {
		return sealedUpload{}, false, verificationFallbackReasonChecksumComposite, nil
	}

	dgst := strings.TrimSpace(attributes.TrustedFullObjectSHA256Digest)
	if dgst == "" {
		return sealedUpload{}, false, verificationFallbackReasonChecksumMalformed, nil
	}
	parsedDigest, err := digest.Parse(dgst)
	if err != nil || parsedDigest.Algorithm().String() != "sha256" {
		return sealedUpload{}, false, verificationFallbackReasonChecksumMalformed, nil
	}
	return sealedUpload{
		Digest:    parsedDigest.String(),
		SizeBytes: attributes.SizeBytes,
	}, true, verificationFallbackReasonNone, nil
}

func (s *Service) verifyUploadedObjectByReading(
	ctx context.Context,
	objectKey string,
	expectedSizeBytes int64,
	result verificationResult,
) (verificationResult, error) {
	objectKey = strings.TrimSpace(objectKey)
	reader, err := s.backend.Reader(ctx, objectKey, 0)
	if err != nil {
		return verificationResult{}, fmt.Errorf("failed to open uploaded object for verification: %w", err)
	}

	hasher := sha256.New()
	progressWriter := newVerificationReadProgressWriter(objectKey, expectedSizeBytes, s.now())
	sizeBytes, copyErr := io.Copy(io.MultiWriter(hasher, progressWriter), reader)
	closeErr := reader.Close()
	switch {
	case copyErr != nil:
		return verificationResult{}, fmt.Errorf("failed to read uploaded object for verification: %w", copyErr)
	case closeErr != nil:
		return verificationResult{}, fmt.Errorf("failed to close uploaded object reader: %w", closeErr)
	}

	result.Source = verificationSourceObjectRead
	result.Sealed = sealedUpload{
		Digest:    "sha256:" + hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes: sizeBytes,
	}
	return result, nil
}

type verificationReadProgressWriter struct {
	objectKey         string
	sizeBytes         int64
	startedAt         time.Time
	lastLoggedAt      time.Time
	readBytes         int64
	nextProgressBytes int64
	now               func() time.Time
	emit              func(string, ...any)
}

func newVerificationReadProgressWriter(objectKey string, sizeBytes int64, startedAt time.Time) *verificationReadProgressWriter {
	return &verificationReadProgressWriter{
		objectKey:         strings.TrimSpace(objectKey),
		sizeBytes:         sizeBytes,
		startedAt:         startedAt,
		lastLoggedAt:      startedAt,
		nextProgressBytes: verificationReadProgressStepBytes,
		now:               time.Now,
		emit: func(message string, args ...any) {
			slog.Default().Info(message, args...)
		},
	}
}

func (w *verificationReadProgressWriter) Write(payload []byte) (int, error) {
	w.readBytes += int64(len(payload))
	for w.shouldLogProgress() {
		w.logProgress()
	}
	return len(payload), nil
}

func (w *verificationReadProgressWriter) shouldLogProgress() bool {
	if w.readBytes <= 0 {
		return false
	}
	if w.nextProgressBytes > 0 && w.readBytes >= w.nextProgressBytes {
		return true
	}
	if w.now == nil {
		return false
	}
	return !w.lastLoggedAt.IsZero() && w.now().Sub(w.lastLoggedAt) >= verificationReadProgressInterval
}

func (w *verificationReadProgressWriter) logProgress() {
	now := time.Now()
	if w.now != nil {
		now = w.now()
	}
	elapsed := now.Sub(w.startedAt)
	throughputBytesPerSec := int64(0)
	if elapsed > 0 {
		throughputBytesPerSec = int64(float64(w.readBytes) / elapsed.Seconds())
	}
	for w.nextProgressBytes > 0 && w.readBytes >= w.nextProgressBytes {
		w.nextProgressBytes += verificationReadProgressStepBytes
	}
	w.lastLoggedAt = now

	if w.sizeBytes > 0 {
		progressPercent := int((w.readBytes * 100) / w.sizeBytes)
		if progressPercent > 100 {
			progressPercent = 100
		}
		w.emit(
			"direct upload verification read progress",
			slog.String("objectKey", w.objectKey),
			slog.Int64("readBytes", w.readBytes),
			slog.Int64("sizeBytes", w.sizeBytes),
			slog.Int("progressPercent", progressPercent),
			slog.Int64("durationMs", elapsed.Milliseconds()),
			slog.Int64("throughputBytesPerSec", throughputBytesPerSec),
		)
		return
	}
	w.emit(
		"direct upload verification read progress",
		slog.String("objectKey", w.objectKey),
		slog.Int64("readBytes", w.readBytes),
		slog.Int64("durationMs", elapsed.Milliseconds()),
		slog.Int64("throughputBytesPerSec", throughputBytesPerSec),
	)
}
