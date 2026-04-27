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
	"os/signal"
	"strings"
	"syscall"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

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
		slog.Duration("completed_request_ttl", options.CompletedRequestTTL),
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
	requestSecrets, err = pruneExpiredCompletedRequests(ctx, client, options.RequestNamespace, requestSecrets, now(), options.CompletedRequestTTL)
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

	result, cleanupErr := cleanupRunner(cycleCtx, options.ConfigPath, options.RegistryBinary, options.GCTimeout, policy)
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

	if err := markRequestsCompleted(ctx, client, options.RequestNamespace, activeSecrets, result, time.Now().UTC()); err != nil {
		return true, err
	}
	slog.Default().Info(
		"dmcr garbage collection requests completed",
		slog.Int("request_count", len(activeSecrets)),
		slog.Any("request_names", requestNames),
	)

	return true, nil
}

func listRequestSecrets(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string) ([]corev1.Secret, error) {
	secretList, err := client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list dmcr garbage-collection request secrets: %w", err)
	}
	return secretList.Items, nil
}

func shouldActivateGarbageCollection(secrets []corev1.Secret, now time.Time, activationDelay time.Duration) bool {
	if len(secrets) == 0 {
		return false
	}

	for _, secret := range secrets {
		if !isQueuedRequest(secret) {
			continue
		}
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
		secretCopy.Annotations[phaseAnnotationKey] = phaseArmed

		if _, err := client.CoreV1().Secrets(namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("arm dmcr garbage-collection request %s: %w", secretCopy.Name, err)
		}
	}
	return nil
}
