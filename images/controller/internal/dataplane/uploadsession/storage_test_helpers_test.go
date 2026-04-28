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

package uploadsession

import (
	"context"

	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
)

type fakeStorageReservations struct {
	reserveErr error
	released   []string
	reserved   []int64
}

func (f *fakeStorageReservations) ReserveUpload(_ context.Context, session SessionRecord, sizeBytes int64) error {
	if f.reserveErr != nil {
		return f.reserveErr
	}
	f.reserved = append(f.reserved, sizeBytes)
	_ = session
	return nil
}

func (f *fakeStorageReservations) ReleaseUpload(_ context.Context, session SessionRecord) error {
	f.released = append(f.released, session.SessionID)
	return nil
}

func insufficientStorageError() error {
	return &storagecapacity.InsufficientStorageError{
		RequestedBytes: 128,
		LimitBytes:     100,
		UsedBytes:      80,
		ReservedBytes:  10,
		AvailableBytes: 10,
	}
}
