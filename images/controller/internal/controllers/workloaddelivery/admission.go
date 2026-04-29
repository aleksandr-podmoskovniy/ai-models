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
	"encoding/json"
	"errors"
	"net/http"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const admissionPath = "/mutate-ai-deckhouse-io-workload-delivery"

func setupAdmission(mgr ctrl.Manager) error {
	if mgr == nil {
		return errors.New("manager must not be nil")
	}

	handler := newAdmissionHandler(mgr.GetScheme(), mgr.GetAPIReader())
	mgr.GetWebhookServer().Register(admissionPath, &admission.Webhook{Handler: handler})
	return nil
}

type admissionHandler struct {
	decoder admission.Decoder
	reader  client.Reader
}

func newAdmissionHandler(scheme *runtime.Scheme, reader client.Reader) *admissionHandler {
	return &admissionHandler{
		decoder: admission.NewDecoder(scheme),
		reader:  reader,
	}
}

func (h *admissionHandler) Handle(ctx context.Context, request admission.Request) admission.Response {
	object, err := h.decodeObject(request)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	templates, err := podTemplatesAndHints(object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	shouldGate, err := h.shouldGate(ctx, request, object, templates[0].Template)
	if err != nil {
		return admission.Denied(err.Error())
	}
	if !shouldGate {
		return admission.Allowed("workload delivery admission skipped")
	}
	changed := false
	for _, ref := range templates {
		if modeldelivery.EnsureSchedulingGate(ref.Template) {
			changed = true
		}
		if ref.Commit != nil {
			if err := ref.Commit(); err != nil {
				return admission.Errored(http.StatusInternalServerError, err)
			}
		}
	}
	if !changed {
		return admission.Allowed("workload delivery scheduling gate already present")
	}

	mutated, err := json.Marshal(object)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	return admission.PatchResponseFromRaw(request.Object.Raw, mutated)
}

func (h *admissionHandler) shouldGate(
	ctx context.Context,
	request admission.Request,
	object client.Object,
	template *corev1.PodTemplateSpec,
) (bool, error) {
	references, found, err := h.deliveryReferences(ctx, object)
	if err != nil || !found {
		return false, err
	}
	if request.Operation == admissionv1.Create {
		return true, nil
	}
	if deliverySignalStateFromTemplate(template).empty() {
		return true, nil
	}

	oldObject, found, err := h.decodeOldObject(request)
	if err != nil || !found {
		return found, err
	}
	oldReferences, oldFound, err := h.deliveryReferences(ctx, oldObject)
	if err != nil || !oldFound {
		return true, err
	}
	if usesModelRefsAnnotation(oldObject.GetAnnotations()) != usesModelRefsAnnotation(object.GetAnnotations()) {
		return true, nil
	}
	return !equalReferences(oldReferences, references), nil
}

func (h *admissionHandler) deliveryReferences(ctx context.Context, object client.Object) ([]Reference, bool, error) {
	references, found, err := parseReferences(object.GetAnnotations())
	if err != nil || found || !isRayClusterObject(object) {
		return references, found, err
	}
	source, found, err := h.rayServiceOwner(ctx, object)
	if err != nil || !found {
		return nil, false, err
	}
	return parseReferences(source.GetAnnotations())
}

func (h *admissionHandler) rayServiceOwner(ctx context.Context, rayCluster client.Object) (client.Object, bool, error) {
	ref, found := rayServiceOwner(rayCluster)
	if !found || h.reader == nil {
		return nil, false, nil
	}
	source := newRayServiceObject()
	if err := h.reader.Get(ctx, client.ObjectKey{Namespace: rayCluster.GetNamespace(), Name: ref.Name}, source); err != nil {
		return nil, false, client.IgnoreNotFound(err)
	}
	return source, true, nil
}

func equalReferences(left, right []Reference) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (h *admissionHandler) decodeObject(request admission.Request) (client.Object, error) {
	object, err := objectForKind(request.Kind)
	if err != nil {
		return nil, err
	}
	if err := h.decoder.Decode(request, object); err != nil {
		return nil, err
	}
	return object, nil
}

func (h *admissionHandler) decodeOldObject(request admission.Request) (client.Object, bool, error) {
	if len(request.OldObject.Raw) == 0 {
		return nil, false, nil
	}
	object, err := objectForKind(request.Kind)
	if err != nil {
		return nil, false, err
	}
	if err := h.decoder.DecodeRaw(request.OldObject, object); err != nil {
		return nil, false, err
	}
	return object, true, nil
}

func objectForKind(kind metav1.GroupVersionKind) (client.Object, error) {
	switch {
	case kind.Group == "apps" && kind.Version == "v1" && kind.Kind == "Deployment":
		return &appsv1.Deployment{}, nil
	case kind.Group == "apps" && kind.Version == "v1" && kind.Kind == "StatefulSet":
		return &appsv1.StatefulSet{}, nil
	case kind.Group == "apps" && kind.Version == "v1" && kind.Kind == "DaemonSet":
		return &appsv1.DaemonSet{}, nil
	case kind.Group == rayClusterGVK.Group && kind.Version == rayClusterGVK.Version && kind.Kind == rayClusterGVK.Kind:
		return newRayClusterObject(), nil
	default:
		return nil, errors.New("unsupported workload delivery admission object kind")
	}
}
