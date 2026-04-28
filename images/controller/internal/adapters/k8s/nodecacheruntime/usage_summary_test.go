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
	"testing"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUsageReporterPatchesRuntimePodAnnotation(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "d8-ai-models",
		Name:      "ai-models-node-cache-runtime-worker-a",
	}})
	reporter, err := NewUsageReporter(clientset, "d8-ai-models", "ai-models-node-cache-runtime-worker-a")
	if err != nil {
		t.Fatalf("NewUsageReporter() error = %v", err)
	}

	summary := nodecache.RuntimeUsageSummary{
		Version:        nodecache.RuntimeUsageSummaryVersion,
		NodeName:       "worker-a",
		LimitBytes:     100,
		UsedBytes:      40,
		AvailableBytes: 60,
		UpdatedAt:      time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC),
	}
	if err := reporter.ReportRuntimeUsage(context.Background(), summary); err != nil {
		t.Fatalf("ReportRuntimeUsage() error = %v", err)
	}

	pod, err := clientset.CoreV1().Pods("d8-ai-models").Get(context.Background(), "ai-models-node-cache-runtime-worker-a", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get(Pod) error = %v", err)
	}
	got, found, err := UsageSummaryFromPod(pod)
	if err != nil || !found {
		t.Fatalf("UsageSummaryFromPod() got found=%v err=%v", found, err)
	}
	if got.AvailableBytes != 60 || got.NodeName != "worker-a" {
		t.Fatalf("unexpected summary %#v", got)
	}
}
