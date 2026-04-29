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
	"encoding/json"
	"strings"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestAdmissionHandlerAddsSchedulingGateOnCreate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind metav1.GroupVersionKind
		obj  runtime.Object
		path string
	}{
		{
			name: "Deployment",
			kind: metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			obj:  admissionDeployment("gemma"),
			path: "/spec/template/spec/schedulingGates",
		},
		{
			name: "StatefulSet",
			kind: metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"},
			obj:  admissionStatefulSet("gemma"),
			path: "/spec/template/spec/schedulingGates",
		},
		{
			name: "DaemonSet",
			kind: metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"},
			obj:  admissionDaemonSet("gemma"),
			path: "/spec/template/spec/schedulingGates",
		},
	}

	handler := newAdmissionHandler(testkit.NewScheme(t, appsv1.AddToScheme))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			response := handler.Handle(t.Context(), admissionRequest(admissionv1.Create, tt.kind, tt.obj, nil))
			if !response.Allowed {
				t.Fatalf("expected admission allowed, got %#v", response.Result)
			}
			assertPatchPath(t, response.Patches, tt.path)
			assertPatchMentionsGate(t, response.Patches)
		})
	}
}

func TestAdmissionHandlerSkipsResolvedUnchangedWorkloadOnUpdate(t *testing.T) {
	t.Parallel()

	oldDeployment := admissionDeployment("gemma")
	newDeployment := admissionDeployment("gemma")
	newDeployment.Spec.Template.Annotations = map[string]string{
		modeldelivery.ResolvedDigestAnnotation: testDigest,
	}
	oldDeployment.Spec.Template.Annotations = map[string]string{
		modeldelivery.ResolvedDigestAnnotation: testDigest,
	}

	handler := newAdmissionHandler(testkit.NewScheme(t, appsv1.AddToScheme))
	response := handler.Handle(t.Context(), admissionRequest(
		admissionv1.Update,
		metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		newDeployment,
		oldDeployment,
	))

	if !response.Allowed {
		t.Fatalf("expected admission allowed, got %#v", response.Result)
	}
	if len(response.Patches) != 0 {
		t.Fatalf("expected no patch for unchanged resolved workload, got %#v", response.Patches)
	}
}

func TestAdmissionHandlerGatesReferenceChangeEvenWhenTemplateWasResolved(t *testing.T) {
	t.Parallel()

	oldDeployment := admissionDeployment("gemma")
	oldDeployment.Spec.Template.Annotations = map[string]string{
		modeldelivery.ResolvedDigestAnnotation: testDigest,
	}
	newDeployment := admissionDeployment("llama")
	newDeployment.Spec.Template.Annotations = map[string]string{
		modeldelivery.ResolvedDigestAnnotation: testDigest,
	}

	handler := newAdmissionHandler(testkit.NewScheme(t, appsv1.AddToScheme))
	response := handler.Handle(t.Context(), admissionRequest(
		admissionv1.Update,
		metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		newDeployment,
		oldDeployment,
	))

	if !response.Allowed {
		t.Fatalf("expected admission allowed, got %#v", response.Result)
	}
	assertPatchPath(t, response.Patches, "/spec/template/spec/schedulingGates")
	assertPatchMentionsGate(t, response.Patches)
}

func TestAdmissionHandlerRejectsAmbiguousReference(t *testing.T) {
	t.Parallel()

	deployment := admissionDeployment("gemma")
	deployment.Annotations[ClusterModelAnnotation] = "cluster-gemma"

	handler := newAdmissionHandler(testkit.NewScheme(t, appsv1.AddToScheme))
	response := handler.Handle(t.Context(), admissionRequest(
		admissionv1.Create,
		metav1.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		deployment,
		nil,
	))

	if response.Allowed {
		t.Fatal("expected ambiguous workload reference to be denied")
	}
}

func admissionRequest(
	operation admissionv1.Operation,
	kind metav1.GroupVersionKind,
	object runtime.Object,
	oldObject runtime.Object,
) admission.Request {
	raw := mustRaw(object)
	request := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: operation,
		Kind:      kind,
		Object:    runtime.RawExtension{Raw: raw},
	}}
	if oldObject != nil {
		request.OldObject = runtime.RawExtension{Raw: mustRaw(oldObject)}
	}
	return request
}

func mustRaw(object runtime.Object) []byte {
	raw, err := json.Marshal(object)
	if err != nil {
		panic(err)
	}
	return raw
}

func admissionDeployment(modelName string) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "runtime",
			Namespace:   "team-a",
			Annotations: map[string]string{ModelAnnotation: modelName},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "runtime"}},
			Template: admissionPodTemplate(),
		},
	}
}

func admissionStatefulSet(modelName string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "runtime",
			Namespace:   "team-a",
			Annotations: map[string]string{ModelAnnotation: modelName},
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "runtime"}},
			Template: admissionPodTemplate(),
		},
	}
}

func admissionDaemonSet(modelName string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "runtime",
			Namespace:   "team-a",
			Annotations: map[string]string{ModelAnnotation: modelName},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "runtime"}},
			Template: admissionPodTemplate(),
		},
	}
}

func admissionPodTemplate() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "runtime"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{
			Name:  "runtime",
			Image: "example.com/runtime:dev",
		}}},
	}
}

func assertPatchPath(t *testing.T, patches []jsonpatch.JsonPatchOperation, path string) {
	t.Helper()

	for _, patch := range patches {
		if patch.Path == path {
			return
		}
	}
	t.Fatalf("expected patch path %q, got %#v", path, patches)
}

func assertPatchMentionsGate(t *testing.T, patches []jsonpatch.JsonPatchOperation) {
	t.Helper()

	for _, patch := range patches {
		if strings.Contains(patch.Json(), modeldelivery.SchedulingGateName) {
			return
		}
	}
	t.Fatalf("expected patch to mention scheduling gate %q, got %#v", modeldelivery.SchedulingGateName, patches)
}
