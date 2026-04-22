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
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const ScheduledRequestName = "dmcr-gc-scheduled"

type schedulePlanner struct {
	schedule cron.Schedule
	next     time.Time
}

func newSchedulePlanner(spec string, now time.Time) (*schedulePlanner, error) {
	cleanSpec := strings.TrimSpace(spec)
	if cleanSpec == "" {
		return nil, nil
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(cleanSpec)
	if err != nil {
		return nil, fmt.Errorf("parse dmcr garbage collection schedule: %w", err)
	}

	now = now.UTC()
	return &schedulePlanner{
		schedule: schedule,
		next:     schedule.Next(now),
	}, nil
}

func (p *schedulePlanner) Due(now time.Time) bool {
	if p == nil || p.schedule == nil || p.next.IsZero() {
		return false
	}
	return !p.next.After(now.UTC())
}

func (p *schedulePlanner) MarkTriggered(now time.Time) {
	if p == nil || p.schedule == nil {
		return
	}
	now = now.UTC()
	for !p.next.After(now) {
		p.next = p.schedule.Next(p.next)
	}
}

func (p *schedulePlanner) WaitDuration(now time.Time) time.Duration {
	if p == nil || p.schedule == nil || p.next.IsZero() {
		return 0
	}
	now = now.UTC()
	if !p.next.After(now) {
		return 0
	}
	return p.next.Sub(now)
}

func maybeEnqueueScheduledRequest(
	ctx context.Context,
	client kubernetes.Interface,
	options Options,
	planner *schedulePlanner,
	now time.Time,
) error {
	if planner == nil || !planner.Due(now) {
		return nil
	}

	requestSecrets, err := listRequestSecrets(ctx, client, options.RequestNamespace, options.RequestLabelSelector)
	if err != nil {
		return err
	}
	planner.MarkTriggered(now)
	if len(requestSecrets) > 0 {
		return nil
	}

	if err := ensureScheduledRequest(ctx, client, options.RequestNamespace, now); err != nil {
		return err
	}

	slog.Default().Info(
		"dmcr scheduled garbage collection request queued",
		slog.String("request_name", ScheduledRequestName),
		slog.Time("queued_at", now.UTC()),
	)
	return nil
}

func ensureScheduledRequest(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	queuedAt time.Time,
) error {
	key := ScheduledRequestName
	existing, err := client.CoreV1().Secrets(namespace).Get(ctx, key, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key,
				Namespace: namespace,
				Labels: map[string]string{
					RequestLabelKey: RequestLabelValue,
				},
				Annotations: map[string]string{
					RequestQueuedAtAnnotationKey: queuedAt.UTC().Format(time.RFC3339Nano),
				},
			},
			Type: corev1.SecretTypeOpaque,
		}
		_, err = client.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
		return err
	case err != nil:
		return fmt.Errorf("get scheduled dmcr garbage-collection request: %w", err)
	default:
		secretCopy := existing.DeepCopy()
		if secretCopy.Labels == nil {
			secretCopy.Labels = make(map[string]string, 1)
		}
		secretCopy.Labels[RequestLabelKey] = RequestLabelValue
		if secretCopy.Annotations == nil {
			secretCopy.Annotations = make(map[string]string, 3)
		}
		secretCopy.Annotations[RequestQueuedAtAnnotationKey] = queuedAt.UTC().Format(time.RFC3339Nano)
		delete(secretCopy.Annotations, switchAnnotationKey)
		delete(secretCopy.Annotations, doneAnnotationKey)
		_, err = client.CoreV1().Secrets(namespace).Update(ctx, secretCopy, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update scheduled dmcr garbage-collection request: %w", err)
		}
		return nil
	}
}
