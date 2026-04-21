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

package catalogstatus

import (
	"context"
	"errors"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/testkit"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestEnsureCleanupHandleRetriesOnConflict(t *testing.T) {
	t.Parallel()

	model := testModel()
	scheme := testkit.NewScheme(t)
	baseClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		model,
	)
	kubeClient := &conflictOnceClient{
		Client:           baseClient,
		conflictOnUpdate: true,
	}
	reconciler := &baseReconciler{client: kubeClient}

	handle := cleanuphandle.Handle{
		Kind: cleanuphandle.KindBackendArtifact,
		Artifact: &cleanuphandle.ArtifactSnapshot{
			Kind:   modelsv1alpha1.ModelArtifactLocationKindOCI,
			URI:    "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
			Digest: "sha256:deadbeef",
		},
		Backend: &cleanuphandle.BackendArtifactHandle{
			Reference: "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef",
		},
	}

	updated, err := reconciler.ensureCleanupHandle(context.Background(), model, handle)
	if err != nil {
		t.Fatalf("ensureCleanupHandle() error = %v", err)
	}
	if !updated {
		t.Fatal("expected cleanup handle update to succeed after conflict retry")
	}
	if kubeClient.updateCalls < 2 {
		t.Fatalf("expected retry after conflict, update calls = %d", kubeClient.updateCalls)
	}

	var stored modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &stored); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if _, found, err := cleanuphandle.FromObject(&stored); err != nil || !found {
		t.Fatalf("expected stored cleanup handle, found=%v err=%v", found, err)
	}
}

func TestUpdateStatusRetriesOnConflict(t *testing.T) {
	t.Parallel()

	model := testModel()
	scheme := testkit.NewScheme(t)
	baseClient := testkit.NewFakeClient(
		t,
		scheme,
		[]client.Object{&modelsv1alpha1.Model{}, &modelsv1alpha1.ClusterModel{}},
		model,
	)
	kubeClient := &conflictOnceClient{
		Client:                 baseClient,
		conflictOnStatusUpdate: true,
	}
	reconciler := &baseReconciler{client: kubeClient}

	current := model.Status
	desired := modelsv1alpha1.ModelStatus{
		Phase: modelsv1alpha1.ModelPhaseReady,
	}

	if err := reconciler.updateStatus(context.Background(), model, &current, desired); err != nil {
		t.Fatalf("updateStatus() error = %v", err)
	}
	if kubeClient.statusUpdateCalls < 2 {
		t.Fatalf("expected retry after conflict, status update calls = %d", kubeClient.statusUpdateCalls)
	}

	var stored modelsv1alpha1.Model
	if err := kubeClient.Get(context.Background(), client.ObjectKeyFromObject(model), &stored); err != nil {
		t.Fatalf("Get(model) error = %v", err)
	}
	if got, want := stored.Status.Phase, modelsv1alpha1.ModelPhaseReady; got != want {
		t.Fatalf("stored phase = %q, want %q", got, want)
	}
}

type conflictOnceClient struct {
	client.Client
	conflictOnUpdate       bool
	conflictOnStatusUpdate bool
	updateCalls            int
	statusUpdateCalls      int
}

func (c *conflictOnceClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	c.updateCalls++
	if c.conflictOnUpdate && c.updateCalls == 1 {
		return newConflictError(obj)
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *conflictOnceClient) Status() client.SubResourceWriter {
	return &conflictOnceStatusWriter{
		SubResourceWriter: c.Client.Status(),
		parent:            c,
	}
}

type conflictOnceStatusWriter struct {
	client.SubResourceWriter
	parent *conflictOnceClient
}

func (w *conflictOnceStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.parent.statusUpdateCalls++
	if w.parent.conflictOnStatusUpdate && w.parent.statusUpdateCalls == 1 {
		return newConflictError(obj)
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func newConflictError(obj client.Object) error {
	return apierrors.NewConflict(
		schema.GroupResource{Group: "ai.deckhouse.io", Resource: "models"},
		obj.GetName(),
		errors.New("conflict injected by test"),
	)
}
