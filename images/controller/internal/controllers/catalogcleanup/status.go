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

package catalogcleanup

import (
	"context"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *baseReconciler) failDeleteStatus(
	ctx context.Context,
	object client.Object,
	cause error,
) (ctrl.Result, bool, error) {
	if err := r.updateDeleteStatus(ctx, object, modelsv1alpha1.ModelConditionReasonFailed, cause.Error()); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{RequeueAfter: r.options.RequeueAfter}, true, nil
}

func (r *baseReconciler) updateDeleteStatus(
	ctx context.Context,
	object client.Object,
	reason modelsv1alpha1.ModelConditionReason,
	message string,
) error {
	status, err := modelobject.GetStatus(object)
	if err != nil {
		return err
	}
	status.Phase = modelsv1alpha1.ModelPhaseDeleting
	status.ObservedGeneration = object.GetGeneration()
	status.Conditions = dropDeleteOnlyConditions(status.Conditions)
	apimeta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type:               string(modelsv1alpha1.ModelConditionReady),
		Status:             metav1.ConditionFalse,
		Reason:             string(reason),
		Message:            message,
		ObservedGeneration: object.GetGeneration(),
		LastTransitionTime: metav1.Now(),
	})
	if err := modelobject.SetStatus(object, status); err != nil {
		return err
	}
	return r.client.Status().Update(ctx, object)
}

func dropDeleteOnlyConditions(conditions []metav1.Condition) []metav1.Condition {
	result := make([]metav1.Condition, 0, len(conditions))
	for _, condition := range conditions {
		if modelsv1alpha1.ModelConditionType(condition.Type) == "CleanupCompleted" {
			continue
		}
		result = append(result, condition)
	}
	return result
}
