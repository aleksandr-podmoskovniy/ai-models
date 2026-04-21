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

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/prometheus/client_golang/prometheus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	unknownDeliveryMode   = "Unknown"
	unknownDeliveryReason = "Unknown"
)

type workloadDeliveryCountKey struct {
	namespace      string
	kind           string
	deliveryMode   string
	deliveryReason string
}

type workloadDeliveryPodKey struct {
	namespace      string
	deliveryMode   string
	deliveryReason string
}

type workloadDeliveryInitStateKey struct {
	namespace      string
	deliveryMode   string
	deliveryReason string
	state          string
	reason         string
}

func (c *Collector) collectManagedWorkloadDelivery(ctx context.Context, ch chan<- prometheus.Metric) error {
	workloadCounts := make(map[workloadDeliveryCountKey]int)
	managedPodCounts := make(map[workloadDeliveryPodKey]int)
	readyPodCounts := make(map[workloadDeliveryPodKey]int)
	initStateCounts := make(map[workloadDeliveryInitStateKey]int)

	if err := c.countManagedDeployments(ctx, workloadCounts); err != nil {
		return err
	}
	if err := c.countManagedStatefulSets(ctx, workloadCounts); err != nil {
		return err
	}
	if err := c.countManagedDaemonSets(ctx, workloadCounts); err != nil {
		return err
	}
	if err := c.countManagedCronJobs(ctx, workloadCounts); err != nil {
		return err
	}
	if err := c.countManagedPods(ctx, managedPodCounts, readyPodCounts, initStateCounts); err != nil {
		return err
	}

	reportManagedWorkloadDelivery(ch, workloadCounts, managedPodCounts, readyPodCounts, initStateCounts)
	return nil
}

func (c *Collector) countManagedDeployments(ctx context.Context, counts map[workloadDeliveryCountKey]int) error {
	var list appsv1.DeploymentList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}
	for i := range list.Items {
		incrementManagedWorkloadDeliveryCount(counts, list.Items[i].Namespace, "Deployment", &list.Items[i].Spec.Template)
	}
	return nil
}

func (c *Collector) countManagedStatefulSets(ctx context.Context, counts map[workloadDeliveryCountKey]int) error {
	var list appsv1.StatefulSetList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}
	for i := range list.Items {
		incrementManagedWorkloadDeliveryCount(counts, list.Items[i].Namespace, "StatefulSet", &list.Items[i].Spec.Template)
	}
	return nil
}

func (c *Collector) countManagedDaemonSets(ctx context.Context, counts map[workloadDeliveryCountKey]int) error {
	var list appsv1.DaemonSetList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}
	for i := range list.Items {
		incrementManagedWorkloadDeliveryCount(counts, list.Items[i].Namespace, "DaemonSet", &list.Items[i].Spec.Template)
	}
	return nil
}

func (c *Collector) countManagedCronJobs(ctx context.Context, counts map[workloadDeliveryCountKey]int) error {
	var list batchv1.CronJobList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}
	for i := range list.Items {
		incrementManagedWorkloadDeliveryCount(counts, list.Items[i].Namespace, "CronJob", &list.Items[i].Spec.JobTemplate.Spec.Template)
	}
	return nil
}

func (c *Collector) countManagedPods(
	ctx context.Context,
	managed map[workloadDeliveryPodKey]int,
	ready map[workloadDeliveryPodKey]int,
	initState map[workloadDeliveryInitStateKey]int,
) error {
	var list corev1.PodList
	if err := c.reader.List(ctx, &list, client.UnsafeDisableDeepCopy); err != nil {
		return err
	}
	for i := range list.Items {
		mode, reason, tracked := managedDeliveryLabels(list.Items[i].Annotations)
		if !tracked {
			continue
		}

		key := workloadDeliveryPodKey{
			namespace:      strings.TrimSpace(list.Items[i].Namespace),
			deliveryMode:   mode,
			deliveryReason: reason,
		}
		managed[key]++
		if podReady(&list.Items[i]) {
			ready[key]++
		}

		state, reason, trackedState := managedInitState(&list.Items[i])
		if !trackedState {
			continue
		}
		initState[workloadDeliveryInitStateKey{
			namespace:      key.namespace,
			deliveryMode:   key.deliveryMode,
			deliveryReason: key.deliveryReason,
			state:          state,
			reason:         reason,
		}]++
	}
	return nil
}

func incrementManagedWorkloadDeliveryCount(
	counts map[workloadDeliveryCountKey]int,
	namespace string,
	kind string,
	template *corev1.PodTemplateSpec,
) {
	if template == nil {
		return
	}

	mode, reason, tracked := managedDeliveryLabels(template.Annotations)
	if !tracked {
		return
	}

	key := workloadDeliveryCountKey{
		namespace:      strings.TrimSpace(namespace),
		kind:           kind,
		deliveryMode:   mode,
		deliveryReason: reason,
	}
	counts[key]++
}

func managedDeliveryLabels(annotations map[string]string) (string, string, bool) {
	digest := strings.TrimSpace(annotations[modeldelivery.ResolvedDigestAnnotation])
	mode := strings.TrimSpace(annotations[modeldelivery.ResolvedDeliveryModeAnnotation])
	reason := strings.TrimSpace(annotations[modeldelivery.ResolvedDeliveryReasonAnnotation])
	if digest == "" && mode == "" && reason == "" {
		return "", "", false
	}
	if mode == "" {
		mode = unknownDeliveryMode
	}
	if reason == "" {
		reason = unknownDeliveryReason
	}
	return mode, reason, true
}

func managedInitState(pod *corev1.Pod) (string, string, bool) {
	if pod == nil {
		return "", "", false
	}
	for _, status := range pod.Status.InitContainerStatuses {
		if status.Name != modeldelivery.DefaultInitContainerName {
			continue
		}
		switch {
		case status.State.Waiting != nil:
			return "Waiting", strings.TrimSpace(status.State.Waiting.Reason), true
		case status.State.Running != nil:
			return "Running", "", true
		case status.State.Terminated != nil:
			state := "Succeeded"
			if status.State.Terminated.ExitCode != 0 {
				state = "Failed"
			}
			return state, strings.TrimSpace(status.State.Terminated.Reason), true
		default:
			return "Unknown", "", true
		}
	}
	return "", "", false
}

func reportManagedWorkloadDelivery(
	ch chan<- prometheus.Metric,
	workloadCounts map[workloadDeliveryCountKey]int,
	managedPodCounts map[workloadDeliveryPodKey]int,
	readyPodCounts map[workloadDeliveryPodKey]int,
	initStateCounts map[workloadDeliveryInitStateKey]int,
) {
	for key, count := range workloadCounts {
		ch <- prometheus.MustNewConstMetric(
			workloadDeliveryWorkloadsManagedMetric.desc,
			prometheus.GaugeValue,
			float64(count),
			key.namespace,
			key.kind,
			key.deliveryMode,
			key.deliveryReason,
		)
	}
	for key, count := range managedPodCounts {
		ch <- prometheus.MustNewConstMetric(
			workloadDeliveryPodsManagedMetric.desc,
			prometheus.GaugeValue,
			float64(count),
			key.namespace,
			key.deliveryMode,
			key.deliveryReason,
		)
	}
	for key, count := range readyPodCounts {
		ch <- prometheus.MustNewConstMetric(
			workloadDeliveryPodsReadyMetric.desc,
			prometheus.GaugeValue,
			float64(count),
			key.namespace,
			key.deliveryMode,
			key.deliveryReason,
		)
	}
	for key, count := range initStateCounts {
		ch <- prometheus.MustNewConstMetric(
			workloadDeliveryInitStateMetric.desc,
			prometheus.GaugeValue,
			float64(count),
			key.namespace,
			key.deliveryMode,
			key.deliveryReason,
			key.state,
			key.reason,
		)
	}
}
