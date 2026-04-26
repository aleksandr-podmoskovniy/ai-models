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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type requestResultRecord struct {
	CompletedAt                           string   `json:"completedAt"`
	RequestNames                          []string `json:"requestNames"`
	StaleRepositoryPrefixCount            int      `json:"staleRepositoryPrefixCount"`
	StaleRawPrefixCount                   int      `json:"staleRawPrefixCount"`
	StaleDirectUploadPrefixCount          int      `json:"staleDirectUploadPrefixCount"`
	OpenDirectUploadMultipartUploadCount  int      `json:"openDirectUploadMultipartUploadCount"`
	OpenDirectUploadMultipartPartCount    int      `json:"openDirectUploadMultipartPartCount"`
	StaleDirectUploadMultipartUploadCount int      `json:"staleDirectUploadMultipartUploadCount"`
	RegistryOutput                        string   `json:"registryOutput,omitempty"`
}

const maxResultRegistryOutputBytes = 8192

func pruneExpiredCompletedRequests(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secrets []corev1.Secret,
	now time.Time,
	ttl time.Duration,
) ([]corev1.Secret, error) {
	kept := make([]corev1.Secret, 0, len(secrets))
	for _, secret := range secrets {
		if !completedRequestExpired(secret, now, ttl) {
			kept = append(kept, secret)
			continue
		}
		if err := deleteRequest(ctx, client, namespace, secret.Name); err != nil {
			return nil, err
		}
	}
	return kept, nil
}

func completedRequestExpired(secret corev1.Secret, now time.Time, ttl time.Duration) bool {
	if !isCompletedRequest(secret) {
		return false
	}
	completedAt, err := time.Parse(time.RFC3339Nano, secret.Annotations[completedAtAnnotationKey])
	if err != nil {
		return true
	}
	return !now.UTC().Before(completedAt.Add(ttl))
}

func isCompletedRequest(secret corev1.Secret) bool {
	return secret.Labels[RequestLabelKey] == RequestLabelValue &&
		strings.TrimSpace(secret.Annotations[phaseAnnotationKey]) == phaseDone
}

func hasPendingRequest(secrets []corev1.Secret) bool {
	for _, secret := range secrets {
		if shouldRunGarbageCollection(secret) || isQueuedRequest(secret) {
			return true
		}
	}
	return false
}

func markRequestsCompleted(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	secrets []corev1.Secret,
	result CleanupResult,
	completedAt time.Time,
) error {
	resultPayload, err := json.Marshal(requestResult(result, completedAt, secretNames(secrets)))
	if err != nil {
		return fmt.Errorf("encode dmcr garbage-collection result: %w", err)
	}
	for _, secret := range secrets {
		secretCopy := secret.DeepCopy()
		if secretCopy.Annotations == nil {
			secretCopy.Annotations = make(map[string]string, 4)
		}
		secretCopy.Annotations[phaseAnnotationKey] = phaseDone
		secretCopy.Annotations[completedAtAnnotationKey] = completedAt.UTC().Format(time.RFC3339Nano)
		delete(secretCopy.Annotations, switchAnnotationKey)
		if secretCopy.Data == nil {
			secretCopy.Data = make(map[string][]byte, 1)
		}
		delete(secretCopy.Data, directUploadTokenDataKey)
		secretCopy.Data[resultDataKey] = resultPayload
		if _, err := client.CoreV1().Secrets(namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("mark dmcr garbage-collection request %s completed: %w", secretCopy.Name, err)
		}
	}
	return nil
}

func requestResult(result CleanupResult, completedAt time.Time, requestNames []string) requestResultRecord {
	return requestResultRecord{
		CompletedAt:                           completedAt.UTC().Format(time.RFC3339Nano),
		RequestNames:                          requestNames,
		StaleRepositoryPrefixCount:            len(result.Report.StaleRepositories),
		StaleRawPrefixCount:                   len(result.Report.StaleRawPrefixes),
		StaleDirectUploadPrefixCount:          len(result.Report.StaleDirectUploadPrefixes),
		OpenDirectUploadMultipartUploadCount:  result.Report.StoredDirectUploadMultipartUploadCount,
		OpenDirectUploadMultipartPartCount:    result.Report.StoredDirectUploadMultipartPartCount,
		StaleDirectUploadMultipartUploadCount: len(result.Report.StaleDirectUploadMultipartUploads),
		RegistryOutput:                        boundedResultRegistryOutput(result.RegistryOutput),
	}
}

func boundedResultRegistryOutput(output string) string {
	trimmed := strings.TrimSpace(output)
	if len(trimmed) <= maxResultRegistryOutputBytes {
		return trimmed
	}
	return trimmed[:maxResultRegistryOutputBytes] + "\n...truncated..."
}

func deleteRequest(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	if err := client.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete dmcr garbage-collection request %s: %w", name, err)
	}
	return nil
}
