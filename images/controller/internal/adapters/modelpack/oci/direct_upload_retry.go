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

package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

type directUploadAPIError struct {
	StatusCode int
	Message    string
}

type directUploadTransportError struct {
	err error
}

const (
	directUploadAPIRequestAttempts  = 180
	directUploadAPIInitialRetryWait = 100 * time.Millisecond
	directUploadAPIMaxRetryWait     = 5 * time.Second
)

func (e *directUploadAPIError) Error() string {
	if e == nil {
		return "DMCR direct upload API error"
	}
	return fmt.Sprintf("DMCR direct upload API returned status %d: %s", e.StatusCode, strings.TrimSpace(e.Message))
}

func (e *directUploadTransportError) Error() string {
	if e == nil || e.err == nil {
		return "failed to call DMCR direct upload API"
	}
	return fmt.Sprintf("failed to call DMCR direct upload API: %v", e.err)
}

func (e *directUploadTransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isDirectUploadStatus(err error, statusCode int) bool {
	var apiErr *directUploadAPIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == statusCode
}

func isTransientDirectUploadAPIError(err error) bool {
	var transportErr *directUploadTransportError
	if errors.As(err, &transportErr) {
		return isTransientDirectUploadTransportError(transportErr.err)
	}

	var apiErr *directUploadAPIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusInternalServerError ||
		apiErr.StatusCode == http.StatusBadGateway ||
		apiErr.StatusCode == http.StatusServiceUnavailable ||
		apiErr.StatusCode == http.StatusGatewayTimeout
}

func isTransientDirectUploadTransportError(err error) bool {
	if err == nil {
		return false
	}
	if isPermanentDirectUploadTransportError(err) {
		return false
	}
	return isTransientDirectUploadNetworkError(err)
}

func isPermanentDirectUploadTransportError(err error) bool {
	var certificateVerificationErr *tls.CertificateVerificationError
	if errors.As(err, &certificateVerificationErr) {
		return true
	}
	var unknownAuthorityErr x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthorityErr) {
		return true
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return true
	}
	var certificateInvalidErr x509.CertificateInvalidError
	if errors.As(err, &certificateInvalidErr) {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func isTransientDirectUploadNetworkError(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && (dnsErr.IsTimeout || dnsErr.IsTemporary) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ECONNABORTED) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.EPIPE) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "connection reset by peer")
}

func sleepDirectUploadRetry(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func nextDirectUploadRetryWait(current time.Duration) time.Duration {
	next := current * 2
	if next > directUploadAPIMaxRetryWait {
		return directUploadAPIMaxRetryWait
	}
	return next
}
