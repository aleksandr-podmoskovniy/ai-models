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
	"reflect"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	"github.com/deckhouse/ai-models/controller/internal/support/resourcenames"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) MarkPublishing(ctx context.Context, ownerUID types.UID) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, uploadsessionstate.MarkPublishingSecret)
}

func (s *Service) MarkCompleted(ctx context.Context, ownerUID types.UID) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, uploadsessionstate.MarkCompletedSecret)
}

func (s *Service) MarkFailed(ctx context.Context, ownerUID types.UID, message string) error {
	return s.mutateSessionSecretByOwnerUID(ctx, ownerUID, func(secret *corev1.Secret) error {
		return uploadsessionstate.MarkPublishingFailedSecret(secret, message)
	})
}

func (s *Service) mutateSessionSecretByOwnerUID(
	ctx context.Context,
	ownerUID types.UID,
	mutate func(secret *corev1.Secret) error,
) error {
	if s == nil {
		return errors.New("upload session service must not be nil")
	}
	if strings.TrimSpace(string(ownerUID)) == "" {
		return errors.New("upload session owner UID must not be empty")
	}
	if mutate == nil {
		return errors.New("upload session secret mutation must not be nil")
	}

	name, err := resourcenames.UploadSessionSecretName(ownerUID)
	if err != nil {
		return err
	}

	var secret corev1.Secret
	if err := s.client.Get(ctx, client.ObjectKey{Name: name, Namespace: s.options.Runtime.Namespace}, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := mutate(&secret); err != nil {
		return err
	}
	return s.client.Update(ctx, &secret)
}

func (s *Service) syncMultipartProgress(
	ctx context.Context,
	secret *corev1.Secret,
	session *uploadsessionstate.Session,
) (*corev1.Secret, *uploadsessionstate.Session, error) {
	if secret == nil || session == nil || !shouldSyncMultipartProgress(session, s.options) {
		return secret, session, nil
	}

	key := client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}
	updatedSecret := secret.DeepCopy()
	updatedSession := session
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latestSecret corev1.Secret
		if err := s.client.Get(ctx, key, &latestSecret); err != nil {
			return err
		}

		latestSession, err := uploadsessionstate.SessionFromSecret(&latestSecret)
		if err != nil {
			return err
		}
		if !shouldSyncMultipartProgress(latestSession, s.options) {
			updatedSecret = latestSecret.DeepCopy()
			updatedSession = latestSession
			return nil
		}
		parts, err := s.options.StagingClient.ListMultipartUploadParts(ctx, uploadstagingports.ListMultipartUploadPartsInput{
			Bucket:   s.options.StagingBucket,
			Key:      strings.TrimSpace(latestSession.Multipart.Key),
			UploadID: strings.TrimSpace(latestSession.Multipart.UploadID),
		})
		if err != nil {
			return err
		}
		updatedParts := uploadedPartsFromStaging(parts)
		updatedSecret = latestSecret.DeepCopy()
		updatedSession = sessionWithUploadedParts(latestSession, updatedParts)
		if reflect.DeepEqual(latestSession.Multipart.UploadedParts, updatedParts) {
			return nil
		}
		if err := uploadsessionstate.SetUploadedPartsSecret(&latestSecret, updatedParts); err != nil {
			return err
		}
		if err := s.client.Update(ctx, &latestSecret); err != nil {
			return err
		}
		updatedSecret = latestSecret.DeepCopy()
		return nil
	}); err != nil {
		return nil, nil, err
	}

	return updatedSecret, updatedSession, nil
}

func shouldSyncMultipartProgress(session *uploadsessionstate.Session, options Options) bool {
	if session == nil || session.Multipart == nil {
		return false
	}
	if session.Phase != uploadsessionstate.PhaseUploading {
		return false
	}
	if strings.TrimSpace(options.StagingBucket) == "" || options.StagingClient == nil {
		return false
	}
	if strings.TrimSpace(session.Multipart.Key) == "" || strings.TrimSpace(session.Multipart.UploadID) == "" {
		return false
	}
	return true
}

func uploadedPartsFromStaging(parts []uploadstagingports.UploadedPart) []uploadsessionruntime.UploadedPart {
	if len(parts) == 0 {
		return nil
	}

	result := make([]uploadsessionruntime.UploadedPart, 0, len(parts))
	for _, part := range parts {
		result = append(result, uploadsessionruntime.UploadedPart{
			PartNumber: part.PartNumber,
			ETag:       strings.TrimSpace(part.ETag),
			SizeBytes:  part.SizeBytes,
		})
	}
	return result
}

func sessionWithUploadedParts(
	session *uploadsessionstate.Session,
	parts []uploadsessionruntime.UploadedPart,
) *uploadsessionstate.Session {
	if session == nil || session.Multipart == nil {
		return session
	}

	cloned := *session
	multipart := *session.Multipart
	multipart.UploadedParts = append([]uploadsessionruntime.UploadedPart(nil), parts...)
	cloned.Multipart = &multipart
	return &cloned
}
