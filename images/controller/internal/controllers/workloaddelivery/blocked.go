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
	"context"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DeliveryBlockedReasonAnnotation  = "ai.deckhouse.io/model-delivery-blocked-reason"
	DeliveryBlockedMessageAnnotation = "ai.deckhouse.io/model-delivery-blocked-message"

	deliveryBlockedReasonInvalidSpec = "InvalidSpec"
)

func (r *baseReconciler) blockWorkloadDelivery(
	ctx context.Context,
	object client.Object,
	original client.Object,
	template *corev1.PodTemplateSpec,
	cause error,
) (ctrl.Result, error) {
	changed, err := r.keepManagedDeliveryBlocked(
		ctx,
		object,
		original,
		template,
		deliveryBlockedReasonInvalidSpec,
		cause.Error(),
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		r.recorder.Event(object, "Warning", "ModelDeliveryBlocked", cause.Error())
	}
	r.logger.Info(
		"runtime delivery blocked by workload spec",
		slog.String("namespace", object.GetNamespace()),
		slog.String("name", object.GetName()),
		slog.String("reason", deliveryBlockedReasonInvalidSpec),
	)
	return ctrl.Result{}, nil
}

func setDeliveryBlockedState(template *corev1.PodTemplateSpec, reason, message string) bool {
	if template == nil {
		return false
	}
	reason = strings.TrimSpace(reason)
	message = trimBlockedMessage(message)
	if template.Annotations == nil {
		template.Annotations = map[string]string{}
	}
	changed := setAnnotationValue(template.Annotations, DeliveryBlockedReasonAnnotation, reason)
	if setAnnotationValue(template.Annotations, DeliveryBlockedMessageAnnotation, message) {
		changed = true
	}
	return changed
}

func clearDeliveryBlockedState(template *corev1.PodTemplateSpec) bool {
	if template == nil {
		return false
	}
	changed := false
	var removed bool
	template.Annotations, removed = removeAnnotation(template.Annotations, DeliveryBlockedReasonAnnotation)
	if removed {
		changed = true
	}
	template.Annotations, removed = removeAnnotation(template.Annotations, DeliveryBlockedMessageAnnotation)
	if removed {
		changed = true
	}
	return changed
}

func trimBlockedMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 512 {
		return message
	}
	return message[:512]
}

func setAnnotationValue(annotations map[string]string, key, value string) bool {
	if annotations[key] == value {
		return false
	}
	annotations[key] = value
	return true
}
