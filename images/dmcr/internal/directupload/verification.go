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
	"log"
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

type verificationMethod string

const (
	verificationMethodTrustedBackendSHA256 verificationMethod = "trusted-backend-sha256"
	verificationMethodObjectRead           verificationMethod = "verification-read"
)

type verificationFallbackReason string

const (
	verificationFallbackReasonNone              verificationFallbackReason = ""
	verificationFallbackReasonAttributesError   verificationFallbackReason = "attributes-error"
	verificationFallbackReasonChecksumMissing   verificationFallbackReason = "checksum-missing"
	verificationFallbackReasonChecksumComposite verificationFallbackReason = "checksum-composite"
	verificationFallbackReasonChecksumMalformed verificationFallbackReason = "checksum-malformed"
)

type verificationResult struct {
	Sealed               sealedUpload
	Method               verificationMethod
	FallbackReason       verificationFallbackReason
	BackendChecksumType  string
	BackendSHA256Present bool
	AvailableChecksums   []string
}

func (s *Service) verifyUploadedObject(ctx context.Context, objectKey string, expectedSizeBytes int64) (verificationResult, error) {
	result, err := s.inspectBackendVerification(ctx, objectKey)
	if err != nil {
		result = verificationResult{
			Method:         verificationMethodObjectRead,
			FallbackReason: verificationFallbackReasonAttributesError,
		}
		log.Printf(
			"direct upload verification falling back to object read objectKey=%q fallbackReason=%q error=%v",
			strings.TrimSpace(objectKey),
			result.FallbackReason,
			err,
		)
		return s.verifyUploadedObjectByReading(ctx, objectKey, expectedSizeBytes, result)
	}
	if result.Method == verificationMethodTrustedBackendSHA256 {
		log.Printf(
			"direct upload verification selected trusted backend checksum objectKey=%q digest=%q sizeBytes=%d backendChecksumType=%q backendSHA256Present=%t availableChecksums=%q",
			strings.TrimSpace(objectKey),
			result.Sealed.Digest,
			result.Sealed.SizeBytes,
			result.BackendChecksumType,
			result.BackendSHA256Present,
			strings.Join(result.AvailableChecksums, ","),
		)
		return result, nil
	}

	log.Printf(
		"direct upload verification falling back to object read objectKey=%q fallbackReason=%q backendChecksumType=%q backendSHA256Present=%t availableChecksums=%q",
		strings.TrimSpace(objectKey),
		result.FallbackReason,
		result.BackendChecksumType,
		result.BackendSHA256Present,
		strings.Join(result.AvailableChecksums, ","),
	)
	return s.verifyUploadedObjectByReading(ctx, objectKey, expectedSizeBytes, result)
}

func (s *Service) inspectBackendVerification(ctx context.Context, objectKey string) (verificationResult, error) {
	attributes, err := s.backend.ObjectAttributes(ctx, strings.TrimSpace(objectKey))
	if err != nil {
		return verificationResult{}, err
	}
	if attributes.SizeBytes < 0 {
		return verificationResult{}, fmt.Errorf("trusted backend sizeBytes must not be negative")
	}

	result := verificationResult{
		Method:               verificationMethodObjectRead,
		BackendChecksumType:  strings.ToUpper(strings.TrimSpace(attributes.ReportedChecksumType)),
		BackendSHA256Present: attributes.SHA256ChecksumPresent,
		AvailableChecksums:   slices.Clone(attributes.AvailableChecksumAlgorithms),
	}
	sealed, ok, reason, err := trustedBackendVerification(attributes)
	if err != nil {
		return verificationResult{}, err
	}
	if ok {
		result.Method = verificationMethodTrustedBackendSHA256
		result.Sealed = sealed
		return result, nil
	}
	result.FallbackReason = reason
	return result, nil
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
	reader, err := s.backend.Reader(ctx, strings.TrimSpace(objectKey), 0)
	if err != nil {
		return verificationResult{}, fmt.Errorf("failed to open uploaded object for verification: %w", err)
	}

	hasher := sha256.New()
	progressWriter := newVerificationReadProgressWriter(strings.TrimSpace(objectKey), expectedSizeBytes, s.now())
	sizeBytes, copyErr := io.Copy(io.MultiWriter(hasher, progressWriter), reader)
	closeErr := reader.Close()
	switch {
	case copyErr != nil:
		return verificationResult{}, fmt.Errorf("failed to read uploaded object for verification: %w", copyErr)
	case closeErr != nil:
		return verificationResult{}, fmt.Errorf("failed to close uploaded object reader: %w", closeErr)
	}

	result.Method = verificationMethodObjectRead
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
		emit:              log.Printf,
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
			"direct upload verification read progress objectKey=%q readBytes=%d sizeBytes=%d progressPercent=%d durationMs=%d throughputBytesPerSec=%d",
			w.objectKey,
			w.readBytes,
			w.sizeBytes,
			progressPercent,
			elapsed.Milliseconds(),
			throughputBytesPerSec,
		)
		return
	}
	w.emit(
		"direct upload verification read progress objectKey=%q readBytes=%d durationMs=%d throughputBytesPerSec=%d",
		w.objectKey,
		w.readBytes,
		elapsed.Milliseconds(),
		throughputBytesPerSec,
	)
}
