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
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/ownedresource"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	publicationapp "github.com/deckhouse/ai-models/controller/internal/application/publishplan"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	"github.com/deckhouse/ai-models/controller/internal/publicationartifact"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
	"github.com/deckhouse/ai-models/controller/internal/support/modelobject"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Service struct {
	client  client.Client
	scheme  *runtime.Scheme
	options Options
}

func NewService(client client.Client, scheme *runtime.Scheme, options Options) (*Service, error) {
	if client == nil {
		return nil, errors.New("upload session client must not be nil")
	}
	if scheme == nil {
		return nil, errors.New("upload session scheme must not be nil")
	}

	options = normalizeOptions(options)
	if err := options.Validate(); err != nil {
		return nil, err
	}

	return &Service{client: client, scheme: scheme, options: options}, nil
}

func (s *Service) GetOrCreate(ctx context.Context, owner client.Object, request publicationports.Request) (*publicationports.UploadSessionHandle, bool, error) {
	if owner == nil {
		return nil, false, errors.New("upload session owner must not be nil")
	}

	plan, err := requestPlan(request)
	if err != nil {
		return nil, false, err
	}
	sessionSecret, rawToken, created, err := s.ensureSecret(ctx, owner, request, plan)
	if err != nil {
		return nil, false, err
	}
	session, err := uploadsessionstate.SessionFromSecret(sessionSecret)
	if err != nil {
		if errors.Is(err, uploadsessionstate.ErrTokenHashMissing) {
			sessionSecret, rawToken, created, err = s.recreateLegacySessionSecret(ctx, owner, request, plan, sessionSecret)
			if err != nil {
				return nil, false, err
			}
			session, err = uploadsessionstate.SessionFromSecret(sessionSecret)
		}
		if err != nil {
			return nil, false, err
		}
	}
	session, err = s.ensureExplicitTerminalPhase(ctx, sessionSecret, session)
	if err != nil {
		return nil, false, err
	}
	handle, err := s.buildHandle(ctx, owner, request, sessionSecret, session, rawToken)
	if err != nil {
		return nil, false, err
	}
	return handle, created, nil
}

func (s *Service) ensureSecret(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
	plan publicationapp.UploadSessionPlan,
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
	expectedSizeBytes := int64(0)
	if plan.ExpectedSizeBytes != nil {
		expectedSizeBytes = *plan.ExpectedSizeBytes
	}
	secret, err := uploadsessionstate.NewSecret(uploadsessionstate.SessionSpec{
		Name:                name,
		Namespace:           s.options.Runtime.Namespace,
		Token:               token,
		ExpectedSizeBytes:   expectedSizeBytes,
		StagingKeyPrefix:    stagingPrefix,
		DeclaredInputFormat: plan.DeclaredInputFormat,
		OwnerGeneration:     owner.GetGeneration(),
		ExpiresAt:           time.Now().Add(s.options.TokenTTL).UTC(),
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

func (s *Service) recreateLegacySessionSecret(
	ctx context.Context,
	owner client.Object,
	request publicationports.Request,
	plan publicationapp.UploadSessionPlan,
	secret *corev1.Secret,
) (*corev1.Secret, string, bool, error) {
	if secret == nil {
		return nil, "", false, errors.New("legacy upload session secret must not be nil")
	}
	if err := ownedresource.DeleteAll(ctx, s.client, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: secret.Name, Namespace: secret.Namespace},
	}); err != nil {
		return nil, "", false, err
	}
	recreated, rawToken, created, err := s.ensureSecret(ctx, owner, request, plan)
	if err != nil {
		return nil, "", false, err
	}
	if !created {
		return nil, "", false, errors.New("recreated upload session secret unexpectedly reused an existing object")
	}
	return recreated, rawToken, true, nil
}

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

func requestPlan(request publicationports.Request) (publicationapp.UploadSessionPlan, error) {
	return publicationapp.IssueUploadSession(publicationapp.UploadSessionIssueRequest{
		OwnerUID:       string(request.Owner.UID),
		OwnerKind:      request.Owner.Kind,
		OwnerName:      request.Owner.Name,
		OwnerNamespace: request.Owner.Namespace,
		Identity:       request.Identity,
		InputFormat:    request.Spec.InputFormat,
		Source:         request.Spec.Source,
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
