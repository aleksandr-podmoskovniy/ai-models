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
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

func workloadEventFilter(options modeldelivery.ServiceOptions) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(evt event.CreateEvent) bool {
			return workloadDeliveryInterest(evt.Object, options)
		},
		UpdateFunc: func(evt event.UpdateEvent) bool {
			return workloadDeliveryInterest(evt.ObjectOld, options) || workloadDeliveryInterest(evt.ObjectNew, options)
		},
		DeleteFunc: func(event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(evt event.GenericEvent) bool {
			return workloadDeliveryInterest(evt.Object, options)
		},
	}
}

func workloadDeliveryInterest(object client.Object, options modeldelivery.ServiceOptions) bool {
	if object == nil {
		return false
	}
	if _, found, err := parseReferences(object.GetAnnotations()); err != nil || found {
		return true
	}
	template, _, err := podTemplateAndHints(object)
	if err != nil {
		return false
	}
	return hasManagedTemplateState(template, options)
}
