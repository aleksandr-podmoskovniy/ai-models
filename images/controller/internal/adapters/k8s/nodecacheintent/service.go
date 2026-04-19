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

package nodecacheintent

import (
	"context"
	"errors"
	"reflect"

	intentcontract "github.com/deckhouse/ai-models/controller/internal/nodecacheintent"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client client.Client
}

func NewService(client client.Client) (*Service, error) {
	if client == nil {
		return nil, errors.New("node cache intent service client must not be nil")
	}
	return &Service{client: client}, nil
}

func (s *Service) ApplyConfigMap(
	ctx context.Context,
	namespace string,
	nodeName string,
	intents []intentcontract.ArtifactIntent,
) error {
	if s == nil {
		return errors.New("node cache intent service must not be nil")
	}

	desired, err := DesiredConfigMap(namespace, nodeName, intents)
	if err != nil {
		return err
	}
	current := desired.DeepCopy()
	if err := s.client.Get(ctx, client.ObjectKeyFromObject(desired), current); err != nil {
		if apierrors.IsNotFound(err) {
			return s.client.Create(ctx, desired)
		}
		return err
	}
	if sameConfigMapProjection(current, desired) {
		return nil
	}
	updated := current.DeepCopy()
	updated.Labels = desired.Labels
	updated.Annotations = desired.Annotations
	updated.Data = desired.Data
	return s.client.Update(ctx, updated)
}

func (s *Service) DeleteConfigMap(ctx context.Context, namespace string, nodeName string) error {
	if s == nil {
		return errors.New("node cache intent service must not be nil")
	}

	desired, err := DesiredConfigMap(namespace, nodeName, nil)
	if err != nil {
		return err
	}
	return client.IgnoreNotFound(s.client.Delete(ctx, desired))
}

func sameConfigMapProjection(current, desired *corev1.ConfigMap) bool {
	return reflect.DeepEqual(current.Labels, desired.Labels) &&
		reflect.DeepEqual(current.Annotations, desired.Annotations) &&
		reflect.DeepEqual(current.Data, desired.Data)
}
