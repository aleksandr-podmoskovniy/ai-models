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

package storageaccounting

import (
	"context"
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStoreReserveRejectsOverflow(t *testing.T) {
	store := newTestStore(t, Options{Namespace: "d8-ai-models", LimitBytes: 50})
	owner := storagecapacity.Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := store.Reserve(context.Background(), storagecapacity.Reservation{ID: "upload-1", Owner: owner, SizeBytes: 40}); err != nil {
		t.Fatalf("Reserve(first) error = %v", err)
	}
	if err := store.Reserve(context.Background(), storagecapacity.Reservation{ID: "upload-2", Owner: owner, SizeBytes: 20}); !storagecapacity.IsInsufficientStorage(err) {
		t.Fatalf("Reserve(second) error = %v, want insufficient storage", err)
	}

	usage, err := store.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if usage.ReservedBytes != 40 || usage.AvailableBytes != 10 {
		t.Fatalf("unexpected usage %#v", usage)
	}
}

func TestStoreReleaseAndCommit(t *testing.T) {
	store := newTestStore(t, Options{Namespace: "d8-ai-models", LimitBytes: 100})
	owner := storagecapacity.Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := store.Reserve(context.Background(), storagecapacity.Reservation{ID: "upload-1", Owner: owner, SizeBytes: 40}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := store.CommitPublished(context.Background(), storagecapacity.PublishedArtifact{ID: "uid-1", Owner: owner, SizeBytes: 35}); err != nil {
		t.Fatalf("CommitPublished() error = %v", err)
	}
	if err := store.ReleaseReservation(context.Background(), "upload-1"); err != nil {
		t.Fatalf("ReleaseReservation() error = %v", err)
	}

	usage, err := store.Usage(context.Background())
	if err != nil {
		t.Fatalf("Usage() error = %v", err)
	}
	if usage.UsedBytes != 35 || usage.ReservedBytes != 0 || usage.AvailableBytes != 65 {
		t.Fatalf("unexpected usage %#v", usage)
	}
}

func newTestStore(t *testing.T, options Options) *Store {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}
	store, err := New(fake.NewClientBuilder().WithScheme(scheme).Build(), options)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return store
}
