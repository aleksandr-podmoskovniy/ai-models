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

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) buildHandle(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
	secret *corev1.Secret,
	session *uploadsessionstate.Session,
	rawToken string,
) (*publicationports.UploadSessionHandle, error) {
	if session == nil {
		return nil, errors.New("upload session must not be nil")
	}
	artifactURI, err := publicationartifact.BuildOCIArtifactReference(
		s.options.Runtime.OCIRepositoryPrefix,
		request.Identity,
		request.Owner.UID,
	)
	if err != nil {
		return nil, err
	}

	phase := corev1.PodRunning
	message := ""
	switch session.Phase {
	case uploadsessionstate.PhaseUploaded, uploadsessionstate.PhaseCompleted:
		phase = corev1.PodSucceeded
		if session.StagedHandle != nil {
			message, err = cleanuphandle.Encode(*session.StagedHandle)
			if err != nil {
				return nil, err
			}
		}
	case uploadsessionstate.PhaseFailed, uploadsessionstate.PhaseAborted, uploadsessionstate.PhaseExpired:
		phase = corev1.PodFailed
		message = session.FailureMessage
	}

	uploadStatus := modelsv1alpha1.ModelUploadStatus{}
	if phase == corev1.PodRunning {
		rawToken, err = s.resolveActiveSessionToken(ctx, owner, secret, artifactURI, session, rawToken)
		if err != nil {
			return nil, err
		}
		uploadStatus = buildUploadStatus(artifactURI, s.options, session.Name, rawToken, session.ExpiresAt)
	}

	return publicationports.NewUploadSessionHandle(
		session.Name,
		phase,
		message,
		uploadStatus,
		func(ctx context.Context) error {
			return ownedresource.DeleteAll(ctx, s.client, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: session.Name, Namespace: s.options.Runtime.Namespace},
			})
		},
	), nil
}

func (s *Service) resolveActiveSessionToken(
	ctx context.Context,
	owner client.Object,
	secret *corev1.Secret,
	artifactURI string,
	session *uploadsessionstate.Session,
	rawToken string,
) (string, error) {
	if secret == nil {
		return "", errors.New("upload session secret must not be nil")
	}
	if session == nil {
		return "", errors.New("upload session must not be nil")
	}
	if rawToken != "" {
		return rawToken, nil
	}

	currentStatus, err := modelobject.GetStatus(owner)
	if err != nil {
		return "", err
	}
	if token, ok := tokenFromPersistedUploadStatus(currentStatus.Upload, artifactURI, s.options, session.Name, session.ExpiresAt); ok {
		return token, nil
	}

	rawToken, err = randomToken()
	if err != nil {
		return "", err
	}
	if err := uploadsessionstate.SetToken(secret, rawToken); err != nil {
		return "", err
	}
	if err := s.client.Update(ctx, secret); err != nil {
		return "", err
	}
	return rawToken, nil
}
