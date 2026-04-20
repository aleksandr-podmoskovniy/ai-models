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

	k8snodecacheruntime "github.com/deckhouse/ai-models/controller/internal/adapters/k8s/nodecacheruntime"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var managedNodeCacheRuntimeLabels = client.MatchingLabels{
	k8snodecacheruntime.ManagedLabelKey: k8snodecacheruntime.ManagedLabelValue,
}

func (c *Collector) listNodeCacheRuntimePods(ctx context.Context) ([]corev1.Pod, error) {
	var list corev1.PodList
	options := []client.ListOption{managedNodeCacheRuntimeLabels, client.UnsafeDisableDeepCopy}
	if strings.TrimSpace(c.runtimeNamespace) != "" {
		options = append(options, client.InNamespace(c.runtimeNamespace))
	}
	if err := c.reader.List(ctx, &list, options...); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *Collector) listNodeCacheRuntimePVCs(ctx context.Context) ([]corev1.PersistentVolumeClaim, error) {
	var list corev1.PersistentVolumeClaimList
	options := []client.ListOption{managedNodeCacheRuntimeLabels, client.UnsafeDisableDeepCopy}
	if strings.TrimSpace(c.runtimeNamespace) != "" {
		options = append(options, client.InNamespace(c.runtimeNamespace))
	}
	if err := c.reader.List(ctx, &list, options...); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (c *Collector) listSelectedNodes(ctx context.Context) ([]corev1.Node, error) {
	if len(c.nodeSelectorLabels) == 0 {
		return nil, nil
	}

	var list corev1.NodeList
	if err := c.reader.List(ctx, &list, client.MatchingLabels(c.nodeSelectorLabels), client.UnsafeDisableDeepCopy); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func reportNodeCacheRuntimePod(ch chan<- prometheus.Metric, pod *corev1.Pod) {
	if pod == nil {
		return
	}

	namespace := strings.TrimSpace(pod.Namespace)
	name := strings.TrimSpace(pod.Name)
	nodeName := podNodeName(pod)
	phase := podPhase(pod)

	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePodPhaseMetric.desc,
		prometheus.GaugeValue,
		1,
		namespace,
		name,
		nodeName,
		phase,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePodReadyMetric.desc,
		prometheus.GaugeValue,
		boolToFloat64(podReady(pod)),
		namespace,
		name,
		nodeName,
	)
}

func reportNodeCacheRuntimePVC(ch chan<- prometheus.Metric, pvc *corev1.PersistentVolumeClaim) {
	if pvc == nil {
		return
	}

	namespace := strings.TrimSpace(pvc.Namespace)
	name := strings.TrimSpace(pvc.Name)
	nodeName := pvcNodeName(pvc)
	storageClassName := pvcStorageClassName(pvc)
	requestedBytes := pvcRequestedBytes(pvc)

	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePVCBoundMetric.desc,
		prometheus.GaugeValue,
		boolToFloat64(pvc.Status.Phase == corev1.ClaimBound),
		namespace,
		name,
		nodeName,
		storageClassName,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePVCRequestedBytesMetric.desc,
		prometheus.GaugeValue,
		requestedBytes,
		namespace,
		name,
		nodeName,
		storageClassName,
	)
}

func reportNodeCacheRuntimeSummary(
	ch chan<- prometheus.Metric,
	runtimeNamespace string,
	nodes []corev1.Node,
	pods []corev1.Pod,
	pvcs []corev1.PersistentVolumeClaim,
) {
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimeNodesDesiredMetric.desc,
		prometheus.GaugeValue,
		float64(len(nodes)),
		runtimeNamespace,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePodsManagedMetric.desc,
		prometheus.GaugeValue,
		float64(len(pods)),
		runtimeNamespace,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePodsReadyMetric.desc,
		prometheus.GaugeValue,
		float64(countReadyPods(pods)),
		runtimeNamespace,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePVCsManagedMetric.desc,
		prometheus.GaugeValue,
		float64(len(pvcs)),
		runtimeNamespace,
	)
	ch <- prometheus.MustNewConstMetric(
		nodeCacheRuntimePVCsBoundMetric.desc,
		prometheus.GaugeValue,
		float64(countBoundPVCs(pvcs)),
		runtimeNamespace,
	)
}

func podNodeName(pod *corev1.Pod) string {
	if pod == nil {
		return ""
	}
	nodeName := strings.TrimSpace(pod.Spec.NodeName)
	if nodeName != "" {
		return nodeName
	}
	return strings.TrimSpace(pod.Annotations[k8snodecacheruntime.NodeNameAnnotationKey])
}

func podPhase(pod *corev1.Pod) string {
	if pod == nil || strings.TrimSpace(string(pod.Status.Phase)) == "" {
		return string(corev1.PodUnknown)
	}
	return string(pod.Status.Phase)
}

func podReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func pvcNodeName(pvc *corev1.PersistentVolumeClaim) string {
	if pvc == nil {
		return ""
	}
	return strings.TrimSpace(pvc.Annotations[k8snodecacheruntime.NodeNameAnnotationKey])
}

func pvcStorageClassName(pvc *corev1.PersistentVolumeClaim) string {
	if pvc == nil || pvc.Spec.StorageClassName == nil {
		return ""
	}
	return strings.TrimSpace(*pvc.Spec.StorageClassName)
}

func pvcRequestedBytes(pvc *corev1.PersistentVolumeClaim) float64 {
	if pvc == nil {
		return 0
	}
	quantity, found := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if !found {
		return 0
	}
	return float64(quantity.Value())
}

func countReadyPods(pods []corev1.Pod) int {
	ready := 0
	for i := range pods {
		if podReady(&pods[i]) {
			ready++
		}
	}
	return ready
}

func countBoundPVCs(pvcs []corev1.PersistentVolumeClaim) int {
	bound := 0
	for i := range pvcs {
		if pvcs[i].Status.Phase == corev1.ClaimBound {
			bound++
		}
	}
	return bound
}

func boolToFloat64(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
