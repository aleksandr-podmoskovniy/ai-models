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
	"encoding/json"
	"errors"

	"github.com/deckhouse/ai-models/controller/internal/domain/storagecapacity"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Store struct {
	client  client.Client
	options Options
}

func New(client client.Client, options Options) (*Store, error) {
	if client == nil {
		return nil, errors.New("storage accounting client must not be nil")
	}
	options = options.Normalize()
	if err := options.Validate(); err != nil {
		return nil, err
	}
	return &Store{client: client, options: options}, nil
}

func (s *Store) Enabled() bool {
	return s != nil && s.options.Enabled()
}

func (s *Store) Reserve(ctx context.Context, reservation storagecapacity.Reservation) error {
	if !s.Enabled() {
		return nil
	}
	return s.update(ctx, func(ledger *storagecapacity.Ledger) error {
		return ledger.Reserve(s.options.LimitBytes, reservation)
	})
}

func (s *Store) ReleaseReservation(ctx context.Context, id string) error {
	if !s.Enabled() {
		return nil
	}
	return s.update(ctx, func(ledger *storagecapacity.Ledger) error {
		ledger.ReleaseReservation(id)
		return nil
	})
}

func (s *Store) CommitPublished(ctx context.Context, artifact storagecapacity.PublishedArtifact) error {
	if !s.Enabled() {
		return nil
	}
	return s.update(ctx, func(ledger *storagecapacity.Ledger) error {
		return ledger.CommitPublished(s.options.LimitBytes, artifact)
	})
}

func (s *Store) ReleasePublished(ctx context.Context, id string) error {
	if !s.Enabled() {
		return nil
	}
	return s.update(ctx, func(ledger *storagecapacity.Ledger) error {
		ledger.ReleasePublished(id)
		return nil
	})
}

func (s *Store) Usage(ctx context.Context) (storagecapacity.Usage, error) {
	if !s.Enabled() {
		return storagecapacity.Usage{}, nil
	}
	ledger, _, err := s.load(ctx)
	if err != nil {
		return storagecapacity.Usage{}, err
	}
	return ledger.Usage(s.options.LimitBytes), nil
}

func (s *Store) update(ctx context.Context, mutate func(*storagecapacity.Ledger) error) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ledger, secret, err := s.load(ctx)
		if err != nil {
			return err
		}
		if mutate == nil {
			return errors.New("storage accounting mutation must not be nil")
		}
		if err := mutate(&ledger); err != nil {
			return err
		}
		raw, err := json.Marshal(ledger)
		if err != nil {
			return err
		}
		if secret == nil {
			secret = s.emptySecret()
			secret.Data = map[string][]byte{ledgerDataKey: raw}
			err := s.client.Create(ctx, secret)
			if apierrors.IsAlreadyExists(err) {
				return apierrors.NewConflict(schema.GroupResource{Resource: "secrets"}, s.options.SecretName, err)
			}
			return err
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Labels = s.emptySecret().Labels
		secret.Type = corev1.SecretTypeOpaque
		secret.Data[ledgerDataKey] = raw
		return s.client.Update(ctx, secret)
	})
}

func (s *Store) load(ctx context.Context) (storagecapacity.Ledger, *corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: s.options.Namespace, Name: s.options.SecretName}, secret)
	if apierrors.IsNotFound(err) {
		return storagecapacity.Ledger{}, nil, nil
	}
	if err != nil {
		return storagecapacity.Ledger{}, nil, err
	}
	if len(secret.Data[ledgerDataKey]) == 0 {
		return storagecapacity.Ledger{}, secret, nil
	}

	var ledger storagecapacity.Ledger
	if err := json.Unmarshal(secret.Data[ledgerDataKey], &ledger); err != nil {
		return storagecapacity.Ledger{}, nil, err
	}
	return ledger, secret, nil
}

func (s *Store) emptySecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.options.SecretName,
			Namespace: s.options.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":      appName,
				"app.kubernetes.io/component": "storage-accounting",
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}
