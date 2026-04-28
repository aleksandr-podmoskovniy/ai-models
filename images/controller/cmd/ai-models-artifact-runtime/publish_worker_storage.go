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

package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageaccounting"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type publishStorageOwner struct {
	ID        string
	Kind      string
	Name      string
	Namespace string
	UID       string
}

type publishStorageReservation struct {
	store *storageaccounting.Store
	owner publishStorageOwner
}

func newPublishStorageReservation(
	namespace string,
	secretName string,
	limit string,
	owner publishStorageOwner,
) (sourcefetch.RemoteStorageReservation, error) {
	limitBytes, err := cmdsupport.ParseOptionalPositiveBytesQuantity(limit, "storage capacity limit")
	if err != nil || limitBytes == 0 {
		return nil, err
	}
	owner.ID = strings.TrimSpace(owner.ID)
	owner.UID = strings.TrimSpace(owner.UID)
	if owner.ID == "" {
		owner.ID = owner.UID
	}
	if owner.ID == "" || owner.UID == "" {
		return nil, errors.New("storage reservation owner UID must not be empty when storage capacity limit is configured")
	}

	store, err := newStorageAccountingStore(namespace, secretName, limitBytes)
	if err != nil {
		return nil, err
	}
	return publishStorageReservation{store: store, owner: owner}, nil
}

func newStorageAccountingStore(namespace string, secretName string, limitBytes int64) (*storageaccounting.Store, error) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := crclient.New(cfg, crclient.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	return storageaccounting.New(client, storageaccounting.Options{
		Namespace:  namespace,
		SecretName: secretName,
		LimitBytes: limitBytes,
	})
}

func (r publishStorageReservation) ReserveRemoteStorage(ctx context.Context, request sourcefetch.RemoteStorageReservationRequest) error {
	return r.store.Reserve(ctx, storagecapacity.Reservation{
		ID: strings.TrimSpace(r.owner.ID),
		Owner: storagecapacity.Owner{
			Kind:      strings.TrimSpace(r.owner.Kind),
			Name:      strings.TrimSpace(r.owner.Name),
			Namespace: strings.TrimSpace(r.owner.Namespace),
			UID:       strings.TrimSpace(r.owner.UID),
		},
		SizeBytes: request.SizeBytes,
		CreatedAt: time.Now().UTC(),
	})
}
