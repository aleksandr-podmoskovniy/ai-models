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

package storagecapacity

import "testing"

func TestLedgerReserveUsesAvailableCapacity(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.CommitPublished(100, PublishedArtifact{ID: "uid-ready", Owner: owner, SizeBytes: 60}); err != nil {
		t.Fatalf("CommitPublished() error = %v", err)
	}
	if err := ledger.Reserve(100, Reservation{ID: "upload-1", Owner: owner, SizeBytes: 30}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}

	usage := ledger.Usage(100)
	if usage.UsedBytes != 60 || usage.ReservedBytes != 30 || usage.AvailableBytes != 10 {
		t.Fatalf("unexpected usage %#v", usage)
	}
}

func TestLedgerReserveRejectsCapacityOverflow(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.Reserve(50, Reservation{ID: "upload-1", Owner: owner, SizeBytes: 40}); err != nil {
		t.Fatalf("Reserve(first) error = %v", err)
	}
	err := ledger.Reserve(50, Reservation{ID: "upload-2", Owner: owner, SizeBytes: 20})
	if !IsInsufficientStorage(err) {
		t.Fatalf("Reserve(second) error = %v, want insufficient storage", err)
	}

	usage := ledger.Usage(50)
	if usage.ReservedBytes != 40 || usage.AvailableBytes != 10 {
		t.Fatalf("unexpected usage after rejected reserve %#v", usage)
	}
}

func TestLedgerReserveIsIdempotentForSameReservation(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.Reserve(50, Reservation{ID: "upload-1", Owner: owner, SizeBytes: 20}); err != nil {
		t.Fatalf("Reserve(first) error = %v", err)
	}
	if err := ledger.Reserve(50, Reservation{ID: "upload-1", Owner: owner, SizeBytes: 20}); err != nil {
		t.Fatalf("Reserve(replay) error = %v", err)
	}

	if got := ledger.Usage(50).ReservedBytes; got != 20 {
		t.Fatalf("reserved bytes = %d, want 20", got)
	}
}

func TestLedgerCommitPublishedReplacingReservationsIsAtomic(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.Reserve(100, Reservation{ID: "uid-1", Owner: owner, SizeBytes: 70}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := ledger.CommitPublishedReplacingReservations(100, []string{"uid-1"}, PublishedArtifact{
		ID:        "uid-1",
		Owner:     owner,
		SizeBytes: 70,
	}); err != nil {
		t.Fatalf("CommitPublishedReplacingReservations() error = %v", err)
	}

	usage := ledger.Usage(100)
	if usage.UsedBytes != 70 || usage.ReservedBytes != 0 || usage.AvailableBytes != 30 {
		t.Fatalf("unexpected promoted usage %#v", usage)
	}
	if _, ok := ledger.Reservations["uid-1"]; ok {
		t.Fatalf("reservation was not removed")
	}
}

func TestLedgerCommitPublishedReplacingReservationsRejectsArtifactGrowth(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.CommitPublished(100, PublishedArtifact{ID: "existing", Owner: owner, SizeBytes: 20}); err != nil {
		t.Fatalf("CommitPublished(existing) error = %v", err)
	}
	if err := ledger.Reserve(100, Reservation{ID: "uid-1", Owner: owner, SizeBytes: 70}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err := ledger.CommitPublishedReplacingReservations(100, []string{"uid-1"}, PublishedArtifact{
		ID:        "uid-1",
		Owner:     owner,
		SizeBytes: 90,
	})
	if !IsInsufficientStorage(err) {
		t.Fatalf("CommitPublishedReplacingReservations() error = %v, want insufficient storage", err)
	}

	usage := ledger.Usage(100)
	if usage.UsedBytes != 20 || usage.ReservedBytes != 70 {
		t.Fatalf("unexpected usage after rejected promote %#v", usage)
	}
}

func TestLedgerReleasePaths(t *testing.T) {
	ledger := Ledger{}
	owner := Owner{Kind: "Model", Name: "llm", Namespace: "team-a", UID: "uid-1"}

	if err := ledger.Reserve(100, Reservation{ID: "upload-1", Owner: owner, SizeBytes: 20}); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := ledger.CommitPublished(100, PublishedArtifact{ID: "uid-1", Owner: owner, SizeBytes: 30}); err != nil {
		t.Fatalf("CommitPublished() error = %v", err)
	}

	ledger.ReleaseReservation("upload-1")
	ledger.ReleasePublished("uid-1")

	if usage := ledger.Usage(100); usage.UsedBytes != 0 || usage.ReservedBytes != 0 || usage.AvailableBytes != 100 {
		t.Fatalf("unexpected usage after release %#v", usage)
	}
}
