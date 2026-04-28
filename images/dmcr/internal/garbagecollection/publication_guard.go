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
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const publicationAppName = "ai-models-publication"

type gcDeferral struct {
	ActivePublicationPodCount int
}

func garbageCollectionDeferral(ctx context.Context, client kubernetes.Interface, namespace string) (gcDeferral, error) {
	count, err := activePublicationPodCount(ctx, client, namespace)
	if err != nil {
		return gcDeferral{}, err
	}
	return gcDeferral{ActivePublicationPodCount: count}, nil
}

func activePublicationPodCount(ctx context.Context, client kubernetes.Interface, namespace string) (int, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: appNameLabelKey + "=" + publicationAppName,
	})
	if err != nil {
		return 0, fmt.Errorf("list active publication pods: %w", err)
	}

	count := 0
	for _, pod := range pods.Items {
		switch pod.Status.Phase {
		case corev1.PodPending, corev1.PodRunning, corev1.PodUnknown:
			count++
		}
	}
	return count, nil
}

func logGarbageCollectionDeferred(reason string, requests []corev1.Secret, deferral gcDeferral) {
	slog.Default().Info(
		"dmcr garbage collection deferred",
		slog.String("reason", reason),
		slog.Int("request_count", len(requests)),
		slog.Any("request_names", secretNames(requests)),
		slog.Int("active_publication_pod_count", deferral.ActivePublicationPodCount),
	)
}
