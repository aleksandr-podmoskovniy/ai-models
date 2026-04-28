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

package nodecacheruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type UsageReporter struct {
	client    kubernetes.Interface
	namespace string
	podName   string
}

func NewUsageReporter(client kubernetes.Interface, namespace, podName string) (*UsageReporter, error) {
	if client == nil {
		return nil, errors.New("node cache runtime usage reporter client must not be nil")
	}
	reporter := &UsageReporter{
		client:    client,
		namespace: strings.TrimSpace(namespace),
		podName:   strings.TrimSpace(podName),
	}
	switch {
	case reporter.namespace == "":
		return nil, fmt.Errorf("%s must not be empty", RuntimePodNamespaceEnv)
	case reporter.podName == "":
		return nil, fmt.Errorf("%s must not be empty", RuntimePodNameEnv)
	default:
		return reporter, nil
	}
}

func NewInClusterUsageReporter() (*UsageReporter, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return NewUsageReporter(
		clientset,
		os.Getenv(RuntimePodNamespaceEnv),
		os.Getenv(RuntimePodNameEnv),
	)
}

func (r *UsageReporter) ReportRuntimeUsage(ctx context.Context, summary nodecache.RuntimeUsageSummary) error {
	if r == nil {
		return errors.New("node cache runtime usage reporter must not be nil")
	}
	value, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	patch, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]string{
				UsageSummaryAnnotation: string(value),
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = r.client.CoreV1().Pods(r.namespace).Patch(ctx, r.podName, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func UsageSummaryFromPod(pod *corev1.Pod) (nodecache.RuntimeUsageSummary, bool, error) {
	if pod == nil {
		return nodecache.RuntimeUsageSummary{}, false, nil
	}
	value := strings.TrimSpace(pod.GetAnnotations()[UsageSummaryAnnotation])
	if value == "" {
		return nodecache.RuntimeUsageSummary{}, false, nil
	}
	var summary nodecache.RuntimeUsageSummary
	if err := json.Unmarshal([]byte(value), &summary); err != nil {
		return nodecache.RuntimeUsageSummary{}, false, err
	}
	if summary.Version != nodecache.RuntimeUsageSummaryVersion {
		return nodecache.RuntimeUsageSummary{}, false, fmt.Errorf("unsupported node cache usage summary version %q", summary.Version)
	}
	if strings.TrimSpace(summary.NodeName) == "" {
		return nodecache.RuntimeUsageSummary{}, false, errors.New("node cache usage summary nodeName must not be empty")
	}
	return summary, true, nil
}
