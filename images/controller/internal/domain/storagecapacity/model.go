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

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Owner struct {
	Kind      string `json:"kind,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}

type Reservation struct {
	ID        string    `json:"id"`
	Owner     Owner     `json:"owner"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type PublishedArtifact struct {
	ID        string    `json:"id"`
	Owner     Owner     `json:"owner"`
	SizeBytes int64     `json:"sizeBytes"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type Ledger struct {
	Version      int                          `json:"version,omitempty"`
	Reservations map[string]Reservation       `json:"reservations,omitempty"`
	Published    map[string]PublishedArtifact `json:"published,omitempty"`
}

type Usage struct {
	CapacityKnown  bool
	LimitBytes     int64
	UsedBytes      int64
	ReservedBytes  int64
	AvailableBytes int64
}

type InsufficientStorageError struct {
	RequestedBytes int64
	LimitBytes     int64
	UsedBytes      int64
	ReservedBytes  int64
	AvailableBytes int64
}

const insufficientStorageErrorPrefix = "artifact storage capacity exceeded"

func (e *InsufficientStorageError) Error() string {
	return fmt.Sprintf(
		insufficientStorageErrorPrefix+": requested=%d available=%d used=%d reserved=%d limit=%d",
		e.RequestedBytes,
		e.AvailableBytes,
		e.UsedBytes,
		e.ReservedBytes,
		e.LimitBytes,
	)
}

func IsInsufficientStorage(err error) bool {
	var typed *InsufficientStorageError
	return errors.As(err, &typed)
}

func IsInsufficientStorageMessage(message string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(message)), insufficientStorageErrorPrefix)
}

func (l *Ledger) Reserve(limitBytes int64, reservation Reservation) error {
	if err := validateEntry(reservation.ID, reservation.Owner, reservation.SizeBytes, "reservation"); err != nil {
		return err
	}
	l.ensure()
	if reservation.CreatedAt.IsZero() {
		reservation.CreatedAt = time.Now().UTC()
	}

	current := l.Reservations[reservation.ID].SizeBytes
	usage := l.Usage(limitBytes)
	nextReserved := usage.ReservedBytes - positive(current) + reservation.SizeBytes
	nextAvailable := limitBytes - usage.UsedBytes - nextReserved
	if limitBytes > 0 && nextAvailable < 0 {
		return &InsufficientStorageError{
			RequestedBytes: reservation.SizeBytes,
			LimitBytes:     limitBytes,
			UsedBytes:      usage.UsedBytes,
			ReservedBytes:  usage.ReservedBytes - positive(current),
			AvailableBytes: maxInt64(0, limitBytes-usage.UsedBytes-(usage.ReservedBytes-positive(current))),
		}
	}

	l.Reservations[reservation.ID] = reservation
	return nil
}

func (l *Ledger) ReleaseReservation(id string) {
	id = strings.TrimSpace(id)
	if id == "" || len(l.Reservations) == 0 {
		return
	}
	delete(l.Reservations, id)
}

func (l *Ledger) CommitPublished(limitBytes int64, artifact PublishedArtifact) error {
	return l.CommitPublishedReplacingReservations(limitBytes, nil, artifact)
}

func (l *Ledger) CommitPublishedReplacingReservations(
	limitBytes int64,
	reservationIDs []string,
	artifact PublishedArtifact,
) error {
	if err := validateEntry(artifact.ID, artifact.Owner, artifact.SizeBytes, "published artifact"); err != nil {
		return err
	}
	l.ensure()
	if artifact.UpdatedAt.IsZero() {
		artifact.UpdatedAt = time.Now().UTC()
	}

	usage := l.Usage(limitBytes)
	releasedReserved := l.reservedBytes(reservationIDs)
	existingPublished := positive(l.Published[artifact.ID].SizeBytes)
	nextUsed := usage.UsedBytes - existingPublished + artifact.SizeBytes
	nextReserved := usage.ReservedBytes - releasedReserved
	if limitBytes > 0 && nextUsed+nextReserved > limitBytes {
		return &InsufficientStorageError{
			RequestedBytes: artifact.SizeBytes,
			LimitBytes:     limitBytes,
			UsedBytes:      usage.UsedBytes - existingPublished,
			ReservedBytes:  nextReserved,
			AvailableBytes: maxInt64(0, limitBytes-(usage.UsedBytes-existingPublished)-nextReserved),
		}
	}

	for _, id := range reservationIDs {
		l.ReleaseReservation(id)
	}
	l.Published[artifact.ID] = artifact
	return nil
}

func (l *Ledger) ReleasePublished(id string) {
	id = strings.TrimSpace(id)
	if id == "" || len(l.Published) == 0 {
		return
	}
	delete(l.Published, id)
}

func (l Ledger) Usage(limitBytes int64) Usage {
	used := sumPublished(l.Published)
	reserved := sumReservations(l.Reservations)
	available := int64(0)
	if limitBytes > 0 {
		available = maxInt64(0, limitBytes-used-reserved)
	}
	return Usage{
		CapacityKnown:  limitBytes > 0,
		LimitBytes:     maxInt64(0, limitBytes),
		UsedBytes:      used,
		ReservedBytes:  reserved,
		AvailableBytes: available,
	}
}

func (l *Ledger) ensure() {
	if l.Version == 0 {
		l.Version = 1
	}
	if l.Reservations == nil {
		l.Reservations = map[string]Reservation{}
	}
	if l.Published == nil {
		l.Published = map[string]PublishedArtifact{}
	}
}

func (l Ledger) reservedBytes(ids []string) int64 {
	if len(ids) == 0 || len(l.Reservations) == 0 {
		return 0
	}
	seen := map[string]struct{}{}
	var total int64
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		total += positive(l.Reservations[id].SizeBytes)
	}
	return total
}

func validateEntry(id string, owner Owner, sizeBytes int64, kind string) error {
	switch {
	case strings.TrimSpace(id) == "":
		return fmt.Errorf("%s ID must not be empty", kind)
	case strings.TrimSpace(owner.UID) == "":
		return fmt.Errorf("%s owner UID must not be empty", kind)
	case sizeBytes <= 0:
		return fmt.Errorf("%s size bytes must be positive", kind)
	default:
		return nil
	}
}

func sumReservations(values map[string]Reservation) int64 {
	var total int64
	for _, value := range values {
		total += positive(value.SizeBytes)
	}
	return total
}

func sumPublished(values map[string]PublishedArtifact) int64 {
	var total int64
	for _, value := range values {
		total += positive(value.SizeBytes)
	}
	return total
}

func positive(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
