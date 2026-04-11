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
	"errors"
	"testing"

	deletionapp "github.com/deckhouse/ai-models/controller/internal/application/deletion"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type failingGetClient struct {
	client.Client
	blockedKey client.ObjectKey
	err        error
}

func (c failingGetClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if key == c.blockedKey {
		return c.err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func TestObserveFinalizeDeleteFlowUploadStagingSkipsGarbageCollectionLookup(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	if err := cleanuphandle.SetOnObject(model, cleanuphandle.Handle{
		Kind: cleanuphandle.KindUploadStaging,
		UploadStaging: &cleanuphandle.UploadStagingHandle{
			Bucket:    "ai-models",
			Key:       "raw/uploads/u1/payload.bin",
			FileName:  "payload.bin",
			SizeBytes: 123,
		},
	}); err != nil {
		t.Fatalf("SetOnObject() error = %v", err)
	}

	jobName := cleanupJobName(t, model)
	reconciler, kubeClient := newModelReconciler(t, model, completedJob("d8-ai-models", jobName))
	reconciler.client = failingGetClient{
		Client:     kubeClient,
		blockedKey: garbageCollectionRequestKey("d8-ai-models", model.GetUID()),
		err:        errors.New("unexpected garbage-collection lookup"),
	}

	flow, err := reconciler.observeFinalizeDeleteFlow(context.Background(), model)
	if err != nil {
		t.Fatalf("observeFinalizeDeleteFlow() error = %v", err)
	}
	if flow.runtime.handle.Kind != cleanuphandle.KindUploadStaging {
		t.Fatalf("unexpected handle kind %q", flow.runtime.handle.Kind)
	}
	if flow.decision != (deletionapp.FinalizeDeleteDecision{RemoveFinalizer: true}) {
		t.Fatalf("unexpected decision %#v", flow.decision)
	}
}

func TestNeedsGarbageCollectionObservation(t *testing.T) {
	t.Parallel()

	backendHandle := cleanuphandle.Handle{Kind: cleanuphandle.KindBackendArtifact}
	uploadHandle := cleanuphandle.Handle{Kind: cleanuphandle.KindUploadStaging}

	cases := []struct {
		name    string
		handle  cleanuphandle.Handle
		job     deletionapp.CleanupJobState
		expects bool
	}{
		{
			name:    "backend artifact waits for completed cleanup job",
			handle:  backendHandle,
			job:     deletionapp.CleanupJobStateComplete,
			expects: true,
		},
		{
			name:    "backend artifact running job does not observe gc yet",
			handle:  backendHandle,
			job:     deletionapp.CleanupJobStateRunning,
			expects: false,
		},
		{
			name:    "upload staging never observes gc",
			handle:  uploadHandle,
			job:     deletionapp.CleanupJobStateComplete,
			expects: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := needsGarbageCollectionObservation(tc.handle, tc.job); got != tc.expects {
				t.Fatalf("needsGarbageCollectionObservation() = %v, want %v", got, tc.expects)
			}
		})
	}
}

func TestObserveFinalizeDeleteFlowBuildsRuntimeOnce(t *testing.T) {
	t.Parallel()

	model := newDeletingModel()
	setCleanupHandle(t, model, "registry.internal.local/ai-models/catalog/namespaced/team-a/deepseek-r1@sha256:deadbeef")

	reconciler, _ := newModelReconciler(t, model)
	flow, err := reconciler.observeFinalizeDeleteFlow(context.Background(), model)
	if err != nil {
		t.Fatalf("observeFinalizeDeleteFlow() error = %v", err)
	}
	if !flow.runtime.hasOwner {
		t.Fatal("expected finalize delete flow to prebuild cleanup owner when decision needs it")
	}
	if flow.runtime.owner.Kind != "Model" {
		t.Fatalf("unexpected owner kind %q", flow.runtime.owner.Kind)
	}
	if flow.decision != (deletionapp.FinalizeDeleteDecision{
		CreateJob:     true,
		UpdateStatus:  true,
		StatusReason:  "CleanupPending",
		StatusMessage: "cleanup job created and waiting for completion",
		Requeue:       true,
	}) {
		t.Fatalf("unexpected decision %#v", flow.decision)
	}
}

var _ client.Client = failingGetClient{}
