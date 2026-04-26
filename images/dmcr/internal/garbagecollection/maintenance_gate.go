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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
	"k8s.io/client-go/kubernetes"
)

var errMaintenanceGateAckQuorumNotReady = errors.New("dmcr maintenance gate ack quorum is not ready")

func syncMaintenanceGateMirror(ctx context.Context, client kubernetes.Interface, options Options) error {
	if strings.TrimSpace(options.MaintenanceGateFile) == "" {
		return nil
	}
	mirror, err := maintenance.NewFileMirror(client, options.RequestNamespace, options.MaintenanceGateName, options.MaintenanceGateFile)
	if err != nil {
		return err
	}
	return mirror.Sync(ctx)
}

func syncMaintenanceGateAckMirror(ctx context.Context, client kubernetes.Interface, options Options) error {
	if strings.TrimSpace(options.MaintenanceGateFile) == "" || strings.TrimSpace(options.MaintenanceGateName) == "" {
		return nil
	}
	mirror, err := maintenance.NewAckMirror(
		client,
		options.RequestNamespace,
		options.MaintenanceGateName,
		options.ExecutorIdentity,
		options.MaintenanceGateFile,
		maintenance.RuntimeAckComponents,
		options.MaintenanceGateAckTTL,
	)
	if err != nil {
		return err
	}
	return mirror.Sync(ctx)
}

func activateMaintenanceGate(
	ctx context.Context,
	client kubernetes.Interface,
	options Options,
	requestNames []string,
) (func(context.Context) error, error) {
	name := strings.TrimSpace(options.MaintenanceGateName)
	if name == "" {
		return nil, nil
	}
	gate, err := maintenance.NewLeaseGate(client, options.RequestNamespace, name, options.ExecutorIdentity, options.MaintenanceGateDuration)
	if err != nil {
		return nil, err
	}
	sequence, release, err := gate.Activate(ctx)
	if err != nil {
		return nil, fmt.Errorf("activate dmcr maintenance gate: %w", err)
	}
	if err := syncMaintenanceGateMirror(ctx, client, options); err != nil {
		_ = release(context.Background())
		return nil, err
	}
	if err := waitMaintenanceGateAckQuorum(ctx, client, options, sequence); err != nil {
		_ = release(context.Background())
		_ = syncMaintenanceGateMirror(context.Background(), client, options)
		return nil, err
	}
	slog.Default().Info(
		"dmcr maintenance gate activated",
		slog.String("maintenance_gate", name),
		slog.String("sequence", sequence),
		slog.Any("request_names", requestNames),
	)
	if options.MaintenanceGateAckQuorum <= 0 && options.MaintenanceGateDelay > 0 {
		timer := time.NewTimer(options.MaintenanceGateDelay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			_ = release(context.Background())
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return func(releaseContext context.Context) error {
		err := release(releaseContext)
		if err == nil {
			slog.Default().Info("dmcr maintenance gate released", slog.String("maintenance_gate", name))
		}
		return err
	}, nil
}

func waitMaintenanceGateAckQuorum(ctx context.Context, client kubernetes.Interface, options Options, sequence string) error {
	if options.MaintenanceGateAckQuorum <= 0 {
		return nil
	}
	deadline := time.Now().UTC().Add(options.MaintenanceGateDelay)
	for {
		if err := syncMaintenanceGateAckMirror(ctx, client, options); err != nil {
			slog.Default().Warn("dmcr maintenance gate local ack mirror sync failed", slog.Any("error", err))
		}
		count, err := maintenance.AckQuorumReady(
			ctx,
			client,
			options.RequestNamespace,
			options.MaintenanceGateName,
			sequence,
			options.MaintenanceGateAckQuorum,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		if count >= options.MaintenanceGateAckQuorum {
			slog.Default().Info(
				"dmcr maintenance gate ack quorum reached",
				slog.String("maintenance_gate", options.MaintenanceGateName),
				slog.String("sequence", sequence),
				slog.Int("ack_count", count),
				slog.Int("ack_quorum", options.MaintenanceGateAckQuorum),
			)
			return nil
		}
		if !time.Now().UTC().Before(deadline) {
			return fmt.Errorf("%w: got %d of %d acks for sequence %s", errMaintenanceGateAckQuorumNotReady, count, options.MaintenanceGateAckQuorum, sequence)
		}
		waitDuration := options.MaintenanceGateMirrorInterval
		if waitDuration <= 0 || waitDuration > 500*time.Millisecond {
			waitDuration = 500 * time.Millisecond
		}
		timer := time.NewTimer(waitDuration)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
