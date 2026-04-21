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
	"log/slog"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func runUploadSession(args []string) int {
	flags := cmdsupport.NewFlagSet(commandUploadGateway)

	var listenPort int
	var partURLTTL time.Duration
	var sessionSecretNamespace string
	var stagingBucket string

	flags.IntVar(&listenPort, "listen-port", 8444, "Listen port.")
	flags.DurationVar(&partURLTTL, "part-url-ttl", 15*time.Minute, "Presigned multipart upload part URL TTL.")
	flags.StringVar(&sessionSecretNamespace, "session-secret-namespace", "", "Namespace of upload session secrets.")
	flags.StringVar(&stagingBucket, "staging-bucket", "", "Bucket used for staged uploads.")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	ctx, stop := cmdsupport.SignalContext()
	defer stop()

	logger := slog.Default().With(
		slog.Int("listenPort", listenPort),
		slog.String("sessionSecretNamespace", strings.TrimSpace(sessionSecretNamespace)),
		slog.String("stagingBucket", strings.TrimSpace(stagingBucket)),
	)

	stagingClient, err := s3.New(cmdsupport.UploadStagingS3ConfigFromEnv())
	if err != nil {
		return cmdsupport.CommandError(commandUploadGateway, err)
	}
	sessions, err := uploadsessionstate.NewInCluster(strings.TrimSpace(sessionSecretNamespace))
	if err != nil {
		return cmdsupport.CommandError(commandUploadGateway, err)
	}

	logger.Info("upload gateway starting")
	if err := uploadsessionruntime.Serve(ctx, uploadsessionruntime.Options{
		ListenPort:    listenPort,
		PartURLTTL:    partURLTTL,
		StagingBucket: stagingBucket,
		StagingClient: stagingClient,
		Sessions:      sessionStoreAdapter{client: sessions},
	}); err != nil && err != ctx.Err() {
		logger.Error("upload gateway failed", slog.Any("error", err))
		return 1
	}

	logger.Info("upload gateway stopped")
	return 0
}

type sessionStoreAdapter struct {
	client *uploadsessionstate.Client
}

func (s sessionStoreAdapter) Load(ctx context.Context, sessionID string) (uploadsessionruntime.SessionRecord, bool, error) {
	session, found, err := s.client.Load(ctx, sessionID)
	if err != nil || !found {
		return uploadsessionruntime.SessionRecord{}, found, err
	}
	record := uploadsessionruntime.SessionRecord{
		SessionID:           session.Name,
		UploadTokenHash:     session.UploadTokenHash,
		ExpectedSizeBytes:   session.ExpectedSizeBytes,
		StagingKeyPrefix:    session.StagingKeyPrefix,
		DeclaredInputFormat: session.DeclaredInputFormat,
		OwnerUID:            session.OwnerUID,
		OwnerKind:           session.OwnerKind,
		OwnerName:           session.OwnerName,
		OwnerNamespace:      session.OwnerNamespace,
		OwnerGeneration:     session.OwnerGeneration,
		ExpiresAt:           session.ExpiresAt.Time,
		FailureMessage:      session.FailureMessage,
	}
	switch session.Phase {
	case uploadsessionstate.PhaseIssued:
		record.Phase = uploadsessionruntime.SessionPhaseIssued
	case uploadsessionstate.PhaseProbing:
		record.Phase = uploadsessionruntime.SessionPhaseProbing
	case uploadsessionstate.PhaseUploading:
		record.Phase = uploadsessionruntime.SessionPhaseUploading
	case uploadsessionstate.PhaseUploaded:
		record.Phase = uploadsessionruntime.SessionPhaseUploaded
	case uploadsessionstate.PhasePublishing:
		record.Phase = uploadsessionruntime.SessionPhasePublishing
	case uploadsessionstate.PhaseCompleted:
		record.Phase = uploadsessionruntime.SessionPhaseCompleted
	case uploadsessionstate.PhaseFailed:
		record.Phase = uploadsessionruntime.SessionPhaseFailed
	case uploadsessionstate.PhaseAborted:
		record.Phase = uploadsessionruntime.SessionPhaseAborted
	case uploadsessionstate.PhaseExpired:
		record.Phase = uploadsessionruntime.SessionPhaseExpired
	}
	if session.Multipart != nil {
		record.Multipart = &uploadsessionruntime.SessionState{
			UploadID:      session.Multipart.UploadID,
			Key:           session.Multipart.Key,
			FileName:      session.Multipart.FileName,
			UploadedParts: session.Multipart.UploadedParts,
		}
	}
	if session.Probe != nil {
		record.Probe = &uploadsessionruntime.ProbeState{
			FileName:            session.Probe.FileName,
			ResolvedInputFormat: session.Probe.ResolvedInputFormat,
		}
	}
	if session.StagedHandle != nil {
		handle := *session.StagedHandle
		record.StagedHandle = &handle
	}
	return record, true, nil
}

func (s sessionStoreAdapter) SaveProbe(ctx context.Context, sessionID string, expectedSizeBytes int64, state uploadsessionruntime.ProbeState) error {
	return s.client.SaveProbe(ctx, sessionID, expectedSizeBytes, state)
}

func (s sessionStoreAdapter) SaveMultipart(ctx context.Context, sessionID string, state uploadsessionruntime.SessionState) error {
	return s.client.SaveMultipart(ctx, sessionID, state)
}

func (s sessionStoreAdapter) SaveMultipartParts(ctx context.Context, sessionID string, parts []uploadsessionruntime.UploadedPart) error {
	return s.client.SaveMultipartParts(ctx, sessionID, parts)
}

func (s sessionStoreAdapter) ClearMultipart(ctx context.Context, sessionID string) error {
	return s.client.ClearMultipart(ctx, sessionID)
}

func (s sessionStoreAdapter) MarkUploaded(ctx context.Context, sessionID string, handle cleanuphandle.Handle) error {
	return s.client.MarkUploaded(ctx, sessionID, handle)
}

func (s sessionStoreAdapter) MarkFailed(ctx context.Context, sessionID string, message string) error {
	return s.client.MarkFailed(ctx, sessionID, message)
}

func (s sessionStoreAdapter) MarkAborted(ctx context.Context, sessionID string, message string) error {
	return s.client.MarkAborted(ctx, sessionID, message)
}

func (s sessionStoreAdapter) MarkExpired(ctx context.Context, sessionID string, message string) error {
	return s.client.MarkExpired(ctx, sessionID, message)
}
