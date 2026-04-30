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

package runtimehealth

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dmcrGCRequestLabelKey        = "ai.deckhouse.io/dmcr-gc-request"
	dmcrGCRequestLabelValue      = "true"
	dmcrGCRequestedAnnotationKey = "ai.deckhouse.io/dmcr-gc-requested-at"
	dmcrGCSwitchAnnotationKey    = "ai.deckhouse.io/dmcr-gc-switch"
	dmcrGCPhaseAnnotationKey     = "ai.deckhouse.io/dmcr-gc-phase"
	dmcrGCCompletedAtKey         = "ai.deckhouse.io/dmcr-gc-completed-at"

	dmcrGCPhaseQueued  = "queued"
	dmcrGCPhaseArmed   = "armed"
	dmcrGCPhaseDone    = "done"
	dmcrGCPhaseUnknown = "unknown"
)

var dmcrGCRequestLabels = client.MatchingLabels{
	dmcrGCRequestLabelKey: dmcrGCRequestLabelValue,
}

func (c *Collector) listDMCRGCRequests(ctx context.Context) ([]corev1.Secret, error) {
	var list corev1.SecretList
	options := []client.ListOption{dmcrGCRequestLabels}
	if strings.TrimSpace(c.cleanupNamespace) != "" {
		options = append(options, client.InNamespace(c.cleanupNamespace))
	}
	if err := c.secretReader.List(ctx, &list, options...); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func reportDMCRGCRequests(
	ch chan<- prometheus.Metric,
	namespace string,
	requests []corev1.Secret,
	now time.Time,
) {
	counts := map[string]int{
		dmcrGCPhaseQueued:  0,
		dmcrGCPhaseArmed:   0,
		dmcrGCPhaseDone:    0,
		dmcrGCPhaseUnknown: 0,
	}
	for i := range requests {
		phase := dmcrGCRequestPhase(&requests[i])
		counts[phase]++
		if age, ok := dmcrGCRequestAge(&requests[i], phase, now); ok {
			ch <- prometheus.MustNewConstMetric(
				dmcrGCRequestAgeSecondsMetric.desc,
				prometheus.GaugeValue,
				age.Seconds(),
				namespace,
				strings.TrimSpace(requests[i].Name),
				phase,
			)
		}
	}
	for _, phase := range []string{dmcrGCPhaseQueued, dmcrGCPhaseArmed, dmcrGCPhaseDone, dmcrGCPhaseUnknown} {
		ch <- prometheus.MustNewConstMetric(
			dmcrGCRequestsMetric.desc,
			prometheus.GaugeValue,
			float64(counts[phase]),
			namespace,
			phase,
		)
	}
}

func dmcrGCRequestPhase(secret *corev1.Secret) string {
	if secret == nil {
		return dmcrGCPhaseUnknown
	}
	switch strings.TrimSpace(secret.Annotations[dmcrGCPhaseAnnotationKey]) {
	case dmcrGCPhaseQueued:
		return dmcrGCPhaseQueued
	case dmcrGCPhaseArmed:
		return dmcrGCPhaseArmed
	case dmcrGCPhaseDone:
		return dmcrGCPhaseDone
	}
	if strings.TrimSpace(secret.Annotations[dmcrGCSwitchAnnotationKey]) != "" {
		return dmcrGCPhaseArmed
	}
	if strings.TrimSpace(secret.Annotations[dmcrGCRequestedAnnotationKey]) != "" {
		return dmcrGCPhaseQueued
	}
	return dmcrGCPhaseUnknown
}

func dmcrGCRequestAge(secret *corev1.Secret, phase string, now time.Time) (time.Duration, bool) {
	if secret == nil {
		return 0, false
	}
	key := dmcrGCRequestedAnnotationKey
	if phase == dmcrGCPhaseDone {
		key = dmcrGCCompletedAtKey
	}
	timestamp := strings.TrimSpace(secret.Annotations[key])
	if timestamp == "" {
		return 0, false
	}
	startedAt, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return 0, false
	}
	age := now.Sub(startedAt)
	if age < 0 {
		return 0, true
	}
	return age, true
}
