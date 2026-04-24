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
	"errors"
	"fmt"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	"github.com/deckhouse/ai-models/controller/internal/domain/ingestadmission"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) materializeSession(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
) (*corev1.Secret, *uploadsessionstate.Session, string, bool, error) {
	sessionSecret, rawToken, created, err := s.ensureSecret(ctx, owner, request)
	if err != nil {
		return nil, nil, "", false, err
	}
	session, err := uploadsessionstate.SessionFromSecret(sessionSecret)
	if err != nil {
		if errors.Is(err, uploadsessionstate.ErrTokenHashMissing) {
			sessionSecret, rawToken, created, err = s.recreateStaleSessionSecret(ctx, owner, request, sessionSecret)
			if err != nil {
				return nil, nil, "", false, err
			}
			session, err = uploadsessionstate.SessionFromSecret(sessionSecret)
		}
		if err != nil {
			return nil, nil, "", false, err
		}
	}
	session, err = s.ensureExplicitTerminalPhase(ctx, sessionSecret, session)
	if err != nil {
		return nil, nil, "", false, err
	}

	return sessionSecret, session, rawToken, created, nil
}

func (s *Service) ensureSecret(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
) (*corev1.Secret, string, bool, error) {
	ownerUID := request.Owner.UID
	name, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return nil, "", false, err
	}
	token, err := randomToken()
	if err != nil {
		return nil, "", false, err
	}
	stagingPrefix, err := resourcenames.UploadStagingObjectPrefix(ownerUID)
	if err != nil {
		return nil, "", false, err
	}
	secret, err := uploadsessionstate.NewSecret(uploadsessionstate.SessionSpec{
		Name:             name,
		Namespace:        s.options.Runtime.Namespace,
		Token:            token,
		StagingKeyPrefix: stagingPrefix,
		OwnerGeneration:  owner.GetGeneration(),
		ExpiresAt:        time.Now().Add(s.options.TokenTTL).UTC(),
	})
	if err != nil {
		return nil, "", false, err
	}
	secret.Labels = mergeMaps(
		secret.Labels,
		resourcenames.OwnerLabels("ai-models-upload-session", request.Owner.Kind, request.Owner.Name, request.Owner.UID, request.Owner.Namespace),
	)
	secret.Annotations = mergeMaps(
		secret.Annotations,
		resourcenames.OwnerAnnotations(request.Owner.Kind, request.Owner.Name, request.Owner.Namespace),
	)

	created, err := ownedresource.CreateOrGet(ctx, s.client, s.scheme, owner, secret)
	if err != nil {
		return nil, "", false, err
	}
	if !created {
		return secret, "", false, nil
	}
	return secret, token, true, nil
}

func (s *Service) recreateStaleSessionSecret(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
	secret *corev1.Secret,
) (*corev1.Secret, string, bool, error) {
	if secret == nil {
		return nil, "", false, errors.New("stale upload session secret must not be nil")
	}
	if err := ownedresource.DeleteAll(ctx, s.client, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace},
	}); err != nil {
		return nil, "", false, err
	}
	recreated, rawToken, created, err := s.ensureSecret(ctx, owner, request)
	if err != nil {
		return nil, "", false, err
	}
	if !created {
		return nil, "", false, errors.New("recreated upload session secret unexpectedly reused an existing object")
	}
	return recreated, rawToken, true, nil
}

func (s *Service) ensureExplicitTerminalPhase(
	ctx context.Context,
	secret *corev1.Secret,
	session *uploadsessionstate.Session,
) (*uploadsessionstate.Session, error) {
	if secret == nil || session == nil || session.ExpiresAt.IsZero() {
		return session, nil
	}
	if session.ExpiresAt.After(time.Now().UTC()) {
		return session, nil
	}
	switch session.Phase {
	case uploadsessionstate.PhaseIssued, uploadsessionstate.PhaseProbing, uploadsessionstate.PhaseUploading:
	default:
		return session, nil
	}

	if err := uploadsessionstate.MarkExpiredSecret(secret, "upload session expired"); err != nil {
		return nil, err
	}
	if err := s.client.Update(ctx, secret); err != nil {
		return nil, err
	}
	session.Phase = uploadsessionstate.PhaseExpired
	session.FailureMessage = "upload session expired"
	return session, nil
}

func validateRequest(request publicationports.Request) error {
	if err := request.Validate(); err != nil {
		return err
	}
	sourceType, err := request.Spec.Source.DetectType()
	if err != nil {
		return err
	}
	if sourceType != modelsv1alpha1.ModelSourceTypeUpload {
		return fmt.Errorf("upload session only supports source type %q", modelsv1alpha1.ModelSourceTypeUpload)
	}
	return ingestadmission.ValidateUploadSession(ingestadmission.UploadSession{
		Owner: ingestadmission.OwnerBinding{
			Kind:      request.Owner.Kind,
			Name:      request.Owner.Name,
			Namespace: request.Owner.Namespace,
			UID:       string(request.Owner.UID),
		},
		Identity: request.Identity,
	})
}

func mergeMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(extra) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]string, len(extra))
	}
	for key, value := range extra {
		base[key] = value
	}
	return base
}
