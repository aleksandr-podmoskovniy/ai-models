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
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/deckhouse/ai-models/dmcr/internal/maintenance"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var errMaintenanceGateAckQuorumNotReady = errors.New("dmcr maintenance gate ack quorum is not ready")

func RunLoop(ctx context.Context, client kubernetes.Interface, options Options) error {
	if client == nil {
		return fmt.Errorf("kubernetes client must not be nil")
	}

	options = applyDefaultOptions(options)
	schedulePlanner, err := newSchedulePlanner(options.Schedule, time.Now().UTC())
	if err != nil {
		return err
	}
	executorLease, err := newExecutorLeaseRunner(client, options, time.Now)
	if err != nil {
		return err
	}
	slog.Default().Info(
		"dmcr garbage collection loop started",
		slog.String("request_namespace", options.RequestNamespace),
		slog.String("request_label_selector", options.RequestLabelSelector),
		slog.Duration("garbage_collection_timeout", options.GCTimeout),
		slog.Duration("rescan_interval", options.RescanInterval),
		slog.Duration("activation_delay", options.ActivationDelay),
		slog.String("schedule", strings.TrimSpace(options.Schedule)),
		slog.String("executor_lease", options.ExecutorLeaseName),
		slog.String("executor_identity", executorLease.identity),
		slog.String("maintenance_gate", options.MaintenanceGateName),
	)

	signalContext, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for {
		if err := syncMaintenanceGateMirror(signalContext, client, options); err != nil {
			slog.Default().Warn("dmcr maintenance gate mirror sync failed", slog.Any("error", err))
		}
		if err := syncMaintenanceGateAckMirror(signalContext, client, options); err != nil {
			slog.Default().Warn("dmcr maintenance gate ack mirror sync failed", slog.Any("error", err))
		}
		handled, err := executorLease.RunIfHolder(signalContext, func(leaseContext context.Context) (bool, error) {
			return runLoopStep(leaseContext, client, options, schedulePlanner, time.Now)
		})
		if err != nil {
			return err
		}
		if !handled {
			waitDuration := options.RescanInterval
			if schedulePlanner != nil {
				if scheduleWait := schedulePlanner.WaitDuration(time.Now().UTC()); scheduleWait > 0 && scheduleWait < waitDuration {
					waitDuration = scheduleWait
				}
			}
			if options.MaintenanceGateMirrorInterval > 0 && options.MaintenanceGateMirrorInterval < waitDuration {
				waitDuration = options.MaintenanceGateMirrorInterval
			}
			select {
			case <-signalContext.Done():
				slog.Default().Info("dmcr garbage collection loop stopped")
				return nil
			case <-time.After(waitDuration):
				continue
			}
		}
	}
}

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

func runLoopStep(
	ctx context.Context,
	client kubernetes.Interface,
	options Options,
	schedulePlanner *schedulePlanner,
	now func() time.Time,
) (bool, error) {
	if err := maybeEnqueueStartupBackfillRequest(ctx, client, options, schedulePlanner, now().UTC()); err != nil {
		return false, err
	}
	if err := maybeEnqueueScheduledRequest(ctx, client, options, schedulePlanner, now().UTC()); err != nil {
		return false, err
	}

	return runRequestCycle(ctx, client, options, func() time.Time {
		return now().UTC()
	})
}

func runRequestCycle(
	ctx context.Context,
	client kubernetes.Interface,
	options Options,
	now func() time.Time,
) (bool, error) {
	requestSecrets, err := listRequestSecrets(ctx, client, options.RequestNamespace, options.RequestLabelSelector)
	if err != nil {
		return false, err
	}
	activeSecrets := activeRequestSecrets(requestSecrets)
	if len(activeSecrets) > 0 {
		return runActiveRequestCycle(ctx, client, options, activeSecrets)
	}

	queuedSecrets := queuedRequestSecrets(requestSecrets)
	if len(queuedSecrets) == 0 {
		return false, nil
	}

	if !shouldActivateGarbageCollection(queuedSecrets, now(), options.ActivationDelay) {
		return false, nil
	}

	if err := armQueuedRequests(ctx, client, options.RequestNamespace, queuedSecrets, now()); err != nil {
		return true, err
	}
	slog.Default().Info(
		"dmcr garbage collection maintenance cycle armed",
		slog.Int("request_count", len(queuedSecrets)),
		slog.Any("request_names", secretNames(queuedSecrets)),
	)
	return true, nil
}

func runActiveRequestCycle(
	ctx context.Context,
	client kubernetes.Interface,
	options Options,
	activeSecrets []corev1.Secret,
) (bool, error) {
	cycleCtx, cancel := context.WithTimeout(ctx, options.GCTimeout)
	defer cancel()

	requestNames := secretNames(activeSecrets)
	policy, err := cleanupPolicyForActiveRequests(options.ConfigPath, activeSecrets)
	if err != nil {
		return true, err
	}
	policy.cleanupStateNamespace = options.RequestNamespace
	releaseGate, err := activateMaintenanceGate(cycleCtx, client, options, requestNames)
	if err != nil {
		if errors.Is(err, errMaintenanceGateAckQuorumNotReady) {
			slog.Default().Warn("dmcr maintenance gate ack quorum is not ready", slog.Any("request_names", requestNames))
			return false, nil
		}
		return true, err
	}
	if releaseGate != nil {
		defer func() {
			if err := releaseGate(context.Background()); err != nil {
				slog.Default().Warn("dmcr maintenance gate release failed", slog.Any("error", err))
			}
			if err := syncMaintenanceGateMirror(context.Background(), client, options); err != nil {
				slog.Default().Warn("dmcr maintenance gate mirror sync failed", slog.Any("error", err))
			}
		}()
	}
	slog.Default().Info(
		"dmcr garbage collection requested",
		slog.Int("request_count", len(activeSecrets)),
		slog.Any("request_names", requestNames),
		slog.Int("targeted_direct_upload_prefix_count", len(policy.targetDirectUploadPrefixes)),
		slog.Int("targeted_direct_upload_multipart_upload_count", len(policy.targetDirectUploadMultipartUploads)),
	)

	result, cleanupErr := autoCleanupRunner(cycleCtx, options.ConfigPath, options.RegistryBinary, options.GCTimeout, policy)
	if cleanupErr != nil {
		return true, cleanupErr
	}

	attrs := []any{
		slog.Int("request_count", len(activeSecrets)),
		slog.Any("request_names", requestNames),
		slog.Int("stale_repository_prefix_count", len(result.Report.StaleRepositories)),
		slog.Int("stale_raw_prefix_count", len(result.Report.StaleRawPrefixes)),
		slog.Int("stale_direct_upload_prefix_count", len(result.Report.StaleDirectUploadPrefixes)),
		slog.Int("open_direct_upload_multipart_upload_count", result.Report.StoredDirectUploadMultipartUploadCount),
		slog.Int("open_direct_upload_multipart_part_count", result.Report.StoredDirectUploadMultipartPartCount),
		slog.Int("stale_direct_upload_multipart_upload_count", len(result.Report.StaleDirectUploadMultipartUploads)),
	}
	if trimmedOutput := strings.TrimSpace(result.RegistryOutput); trimmedOutput != "" {
		attrs = append(attrs, slog.String("registry_output", trimmedOutput))
	}
	slog.Default().Info("dmcr garbage collection completed", attrs...)

	if err := deleteRequests(ctx, client, options.RequestNamespace, activeSecrets); err != nil {
		return true, err
	}
	slog.Default().Info(
		"dmcr garbage collection requests removed",
		slog.Int("request_count", len(activeSecrets)),
		slog.Any("request_names", requestNames),
	)

	return true, nil
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

func listRequestSecrets(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) ([]corev1.Secret, error) {
	secretList, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list dmcr garbage-collection request secrets: %w", err)
	}
	return secretList.Items, nil
}

func queuedRequestSecrets(secrets []corev1.Secret) []corev1.Secret {
	queued := make([]corev1.Secret, 0, len(secrets))
	for _, secret := range secrets {
		if isQueuedRequest(secret) {
			queued = append(queued, secret)
		}
	}
	return queued
}

func activeRequestSecrets(secrets []corev1.Secret) []corev1.Secret {
	active := make([]corev1.Secret, 0, len(secrets))
	for _, secret := range secrets {
		if shouldRunGarbageCollection(secret) {
			active = append(active, secret)
		}
	}
	return active
}

func shouldRunGarbageCollection(secret corev1.Secret) bool {
	if secret.Labels[RequestLabelKey] != RequestLabelValue {
		return false
	}
	return strings.TrimSpace(secret.Annotations[switchAnnotationKey]) != ""
}

func isQueuedRequest(secret corev1.Secret) bool {
	if secret.Labels[RequestLabelKey] != RequestLabelValue {
		return false
	}
	if strings.TrimSpace(secret.Annotations[switchAnnotationKey]) != "" {
		return false
	}
	return strings.TrimSpace(secret.Annotations[RequestQueuedAtAnnotationKey]) != ""
}

func secretNames(secrets []corev1.Secret) []string {
	names := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		names = append(names, secret.Name)
	}
	return names
}

func shouldActivateGarbageCollection(secrets []corev1.Secret, now time.Time, activationDelay time.Duration) bool {
	if len(secrets) == 0 {
		return false
	}

	for _, secret := range secrets {
		requestedAt, err := time.Parse(time.RFC3339Nano, secret.Annotations[RequestQueuedAtAnnotationKey])
		if err != nil {
			return true
		}
		if now.Sub(requestedAt) >= activationDelay {
			return true
		}
	}

	return false
}

func execGarbageCollect(ctx context.Context, options Options) ([]byte, error) {
	gcContext, cancel := context.WithTimeout(ctx, options.GCTimeout)
	defer cancel()

	command := exec.CommandContext(gcContext, options.RegistryBinary, "garbage-collect", options.ConfigPath, "--delete-untagged")
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run dmcr garbage-collect: %s", message)
	}
	return output, nil
}

func armQueuedRequests(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secrets []corev1.Secret,
	armedAt time.Time,
) error {
	for _, secret := range secrets {
		secretCopy := secret.DeepCopy()
		if secretCopy.Annotations == nil {
			secretCopy.Annotations = make(map[string]string, 3)
		}
		secretCopy.Annotations[switchAnnotationKey] = armedAt.Format(time.RFC3339Nano)

		if _, err := client.CoreV1().Secrets(namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("arm dmcr garbage-collection request %s: %w", secretCopy.Name, err)
		}
	}
	return nil
}

func deleteRequests(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secrets []corev1.Secret,
) error {
	for _, secret := range secrets {
		if err := client.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("delete dmcr garbage-collection request %s: %w", secret.Name, err)
		}
	}
	return nil
}
