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

package workloaddelivery

import (
	"log/slog"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) recordDeliveryGate(
	object client.Object,
	result modeldelivery.ApplyResult,
	resolution Resolution,
) bool {
	if result.GateReason == "" {
		return false
	}
	eventType, eventReason := deliveryGateEvent(result.GateReason)
	message := result.GateMessage
	if message == "" {
		message = "Runtime delivery is waiting for node-cache readiness"
	}
	r.logger.Info(
		"runtime delivery gated",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("artifactDigest", resolution.Artifact.Digest),
		slog.Int("modelCount", resolution.modelCount()),
		slog.String("topologyKind", string(result.TopologyKind)),
		slog.String("deliveryMode", string(result.DeliveryMode)),
		slog.String("deliveryReason", string(result.DeliveryReason)),
		slog.String("gateReason", string(result.GateReason)),
		slog.String("gateMessage", message),
	)
	r.recorder.Event(object, eventType, eventReason, message)
	return true
}

func deliveryGateEvent(reason modeldelivery.DeliveryGateReason) (string, string) {
	if reason == modeldelivery.DeliveryGateReasonInsufficientNodeCacheCapacity {
		return "Warning", "ModelDeliveryBlocked"
	}
	return "Normal", "ModelDeliveryPending"
}
