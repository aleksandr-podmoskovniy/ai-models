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

package modelobject

import (
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/publication"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestPublicationRequestForModel(t *testing.T) {
	t.Parallel()

	object := &modelsv1alpha1.Model{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gemma",
			Namespace: "team-a",
			UID:       types.UID("uid-1"),
		},
	}

	request, err := PublicationRequest(object, modelsv1alpha1.ModelSpec{})
	if err != nil {
		t.Fatalf("PublicationRequest() error = %v", err)
	}
	if request.Identity.Scope != publication.ScopeNamespaced {
		t.Fatalf("expected namespaced scope, got %q", request.Identity.Scope)
	}
	if request.Owner.Namespace != "team-a" {
		t.Fatalf("expected namespace in owner, got %#v", request.Owner)
	}
}

func TestPublicationRequestForClusterModel(t *testing.T) {
	t.Parallel()

	object := &modelsv1alpha1.ClusterModel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gemma",
			UID:  types.UID("uid-2"),
		},
	}

	request, err := PublicationRequest(object, modelsv1alpha1.ModelSpec{})
	if err != nil {
		t.Fatalf("PublicationRequest() error = %v", err)
	}
	if request.Identity.Scope != publication.ScopeCluster {
		t.Fatalf("expected cluster scope, got %q", request.Identity.Scope)
	}
	if request.Owner.Namespace != "" {
		t.Fatalf("expected empty owner namespace, got %#v", request.Owner)
	}
}

func TestStatusRoundTrip(t *testing.T) {
	t.Parallel()

	object := &modelsv1alpha1.Model{}
	status := modelsv1alpha1.ModelStatus{Phase: modelsv1alpha1.ModelPhaseReady}
	if err := SetStatus(object, status); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}

	got, err := GetStatus(object)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if got.Phase != modelsv1alpha1.ModelPhaseReady {
		t.Fatalf("expected ready phase, got %#v", got)
	}
}
