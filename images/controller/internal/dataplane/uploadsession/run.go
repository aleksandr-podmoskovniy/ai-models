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

package uploadsession

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

const (
	defaultPartURLTTL             = 15 * time.Minute
	minimumMultipartPartSizeBytes = 5 * 1024 * 1024
	maximumMultipartPartCount     = 10000
)

type SessionPhase string

const (
	SessionPhaseIssued     SessionPhase = "issued"
	SessionPhaseProbing    SessionPhase = "probing"
	SessionPhaseUploading  SessionPhase = "uploading"
	SessionPhaseUploaded   SessionPhase = "uploaded"
	SessionPhasePublishing SessionPhase = "publishing"
	SessionPhaseCompleted  SessionPhase = "completed"
	SessionPhaseFailed     SessionPhase = "failed"
	SessionPhaseAborted    SessionPhase = "aborted"
	SessionPhaseExpired    SessionPhase = "expired"
)

type UploadedPart struct {
	PartNumber int32
	ETag       string
	SizeBytes  int64
}

type SessionState struct {
	UploadID      string
	Key           string
	FileName      string
	UploadedParts []UploadedPart
}

type ProbeState struct {
	FileName            string
	ResolvedInputFormat modelsv1alpha1.ModelInputFormat
}

type SessionRecord struct {
	SessionID           string
	UploadTokenHash     string
	ExpectedSizeBytes   int64
	StagingKeyPrefix    string
	DeclaredInputFormat modelsv1alpha1.ModelInputFormat
	OwnerUID            string
	OwnerKind           string
	OwnerName           string
	OwnerNamespace      string
	OwnerGeneration     int64
	ExpiresAt           time.Time
	Phase               SessionPhase
	Probe               *ProbeState
	Multipart           *SessionState
	FailureMessage      string
	StagedHandle        *cleanuphandle.Handle
}

type SessionStore interface {
	Load(ctx context.Context, sessionID string) (SessionRecord, bool, error)
	SaveProbe(ctx context.Context, sessionID string, expectedSizeBytes int64, state ProbeState) error
	SaveMultipart(ctx context.Context, sessionID string, state SessionState) error
	SaveMultipartParts(ctx context.Context, sessionID string, parts []UploadedPart) error
	ClearMultipart(ctx context.Context, sessionID string) error
	MarkUploaded(ctx context.Context, sessionID string, handle cleanuphandle.Handle) error
	MarkFailed(ctx context.Context, sessionID string, message string) error
	MarkAborted(ctx context.Context, sessionID string, message string) error
	MarkExpired(ctx context.Context, sessionID string, message string) error
}

type Options struct {
	ListenPort          int
	PartURLTTL          time.Duration
	StagingBucket       string
	StagingClient       uploadstagingports.Client
	Sessions            SessionStore
	StorageReservations StorageReservations
}

type sessionAPI struct {
	options Options
	mu      sync.Mutex
}

func Serve(ctx context.Context, options Options) error {
	options = normalizeOptions(options)
	if err := validateOptions(options); err != nil {
		return err
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", normalizePort(options.ListenPort)),
		Handler: newHandler(&sessionAPI{options: options}),
	}

	serverErrCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			serverErrCh <- nil
			return
		}
		serverErrCh <- err
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return ctx.Err()
	case err := <-serverErrCh:
		return err
	}
}

func normalizeOptions(options Options) Options {
	if options.PartURLTTL <= 0 {
		options.PartURLTTL = defaultPartURLTTL
	}
	return options
}

func validateOptions(options Options) error {
	switch {
	case strings.TrimSpace(options.StagingBucket) == "":
		return errors.New("staging bucket must not be empty")
	case options.StagingClient == nil:
		return errors.New("staging client must not be nil")
	case options.Sessions == nil:
		return errors.New("session store must not be nil")
	case options.PartURLTTL <= 0:
		return errors.New("part URL ttl must be positive")
	default:
		return nil
	}
}

func normalizePort(port int) int {
	if port <= 0 {
		return 8444
	}
	return port
}
