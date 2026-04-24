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

package garbagecollection

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

type fakePrefixObject struct {
	key          string
	lastModified time.Time
	payload      []byte
}

type fakeMultipartUpload struct {
	key         string
	uploadID    string
	initiatedAt time.Time
	partCount   int
}

type fakePrefixStore struct {
	objects                 map[string]fakePrefixObject
	multipartUploads        map[directUploadMultipartTarget]fakeMultipartUpload
	multipartUploadPartErrs map[directUploadMultipartTarget]error
	deletedPrefixes         []string
	abortedMultipartUploads []directUploadMultipartTarget
}

func newFakePrefixStore(objects ...fakePrefixObject) *fakePrefixStore {
	result := &fakePrefixStore{
		objects:                 make(map[string]fakePrefixObject, len(objects)),
		multipartUploads:        make(map[directUploadMultipartTarget]fakeMultipartUpload),
		multipartUploadPartErrs: make(map[directUploadMultipartTarget]error),
	}
	for _, object := range objects {
		result.objects[strings.Trim(strings.TrimSpace(object.key), "/")] = object
	}
	return result
}

func newFakePrefixStoreWithMultipartUploads(objects []fakePrefixObject, uploads ...fakeMultipartUpload) *fakePrefixStore {
	result := newFakePrefixStore(objects...)
	for _, upload := range uploads {
		target := normalizeDirectUploadMultipartTarget(directUploadMultipartTarget{
			ObjectKey: upload.key,
			UploadID:  upload.uploadID,
		})
		result.multipartUploads[target] = fakeMultipartUpload{
			key:         target.ObjectKey,
			uploadID:    target.UploadID,
			initiatedAt: upload.initiatedAt,
			partCount:   upload.partCount,
		}
	}
	return result
}

type deletingFakePrefixStore struct {
	*fakePrefixStore
}

func newDeletingFakePrefixStore(objects ...fakePrefixObject) *deletingFakePrefixStore {
	return &deletingFakePrefixStore{fakePrefixStore: newFakePrefixStore(objects...)}
}

func (s *fakePrefixStore) ForEachObject(_ context.Context, prefix string, visit func(string)) error {
	keys := s.matchingKeys(prefix)
	for _, key := range keys {
		visit(key)
	}
	return nil
}

func (s *fakePrefixStore) ForEachObjectInfo(_ context.Context, prefix string, visit func(prefixObjectInfo)) error {
	keys := s.matchingKeys(prefix)
	for _, key := range keys {
		object := s.objects[key]
		visit(prefixObjectInfo{
			Key:          key,
			LastModified: object.lastModified,
		})
	}
	return nil
}

func (s *fakePrefixStore) GetObject(_ context.Context, key string) ([]byte, error) {
	object, found := s.objects[strings.Trim(strings.TrimSpace(key), "/")]
	if !found {
		return nil, fmt.Errorf("object %s not found", key)
	}
	return append([]byte(nil), object.payload...), nil
}

func (s *fakePrefixStore) ForEachMultipartUpload(_ context.Context, prefix string, visit func(multipartUploadInfo)) error {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	targets := make([]directUploadMultipartTarget, 0, len(s.multipartUploads))
	for target := range s.multipartUploads {
		if cleanPrefix == "" || target.ObjectKey == cleanPrefix || strings.HasPrefix(target.ObjectKey, cleanPrefix+"/") {
			targets = append(targets, target)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].ObjectKey == targets[j].ObjectKey {
			return targets[i].UploadID < targets[j].UploadID
		}
		return targets[i].ObjectKey < targets[j].ObjectKey
	})
	for _, target := range targets {
		upload := s.multipartUploads[target]
		visit(multipartUploadInfo{
			Key:         upload.key,
			UploadID:    upload.uploadID,
			InitiatedAt: upload.initiatedAt,
		})
	}
	return nil
}

func (s *fakePrefixStore) CountMultipartUploadParts(_ context.Context, objectKey, uploadID string) (int, error) {
	target := normalizeDirectUploadMultipartTarget(directUploadMultipartTarget{
		ObjectKey: objectKey,
		UploadID:  uploadID,
	})
	if err, found := s.multipartUploadPartErrs[target]; found {
		return 0, err
	}
	upload, found := s.multipartUploads[target]
	if !found {
		return 0, fmt.Errorf("multipart upload %s (%s) not found", target.ObjectKey, target.UploadID)
	}
	return upload.partCount, nil
}

func (s *fakePrefixStore) DeletePrefix(_ context.Context, prefix string) error {
	s.deletedPrefixes = append(s.deletedPrefixes, strings.TrimSpace(prefix))
	return nil
}

func (s *fakePrefixStore) AbortMultipartUpload(_ context.Context, objectKey, uploadID string) error {
	target := normalizeDirectUploadMultipartTarget(directUploadMultipartTarget{
		ObjectKey: objectKey,
		UploadID:  uploadID,
	})
	s.abortedMultipartUploads = append(s.abortedMultipartUploads, target)
	delete(s.multipartUploads, target)
	return nil
}

func (s *deletingFakePrefixStore) DeletePrefix(_ context.Context, prefix string) error {
	listPrefix := strings.TrimLeft(strings.TrimSpace(prefix), "/")
	s.deletedPrefixes = append(s.deletedPrefixes, strings.TrimSpace(prefix))
	for key := range s.objects {
		if listPrefix == "" || strings.HasPrefix(key, listPrefix) {
			delete(s.objects, key)
		}
	}
	return nil
}

func (s *fakePrefixStore) matchingKeys(prefix string) []string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	keys := make([]string, 0, len(s.objects))
	for key := range s.objects {
		if cleanPrefix == "" || key == cleanPrefix || strings.HasPrefix(key, cleanPrefix+"/") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (s *deletingFakePrefixStore) hasObject(key string) bool {
	_, found := s.objects[strings.Trim(strings.TrimSpace(key), "/")]
	return found
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

const legacyCleanupHandleAnnotationKey = "ai.deckhouse.io/cleanup-handle"

func newFakeDynamicClient(t *testing.T, objects ...runtime.Object) *fake.Clientset {
	t.Helper()

	secrets := make([]runtime.Object, 0, len(objects))
	for _, object := range objects {
		if secret := cleanupStateSecretForTest(object); secret != nil {
			secrets = append(secrets, secret)
		}
	}
	return fake.NewSimpleClientset(secrets...)
}

func cleanupStateSecretForTest(object runtime.Object) *corev1.Secret {
	switch typed := object.(type) {
	case *corev1.Secret:
		return typed
	case *unstructured.Unstructured:
		raw := strings.TrimSpace(typed.GetAnnotations()[legacyCleanupHandleAnnotationKey])
		if raw == "" {
			return nil
		}
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cleanup-state-" + strings.TrimSpace(typed.GetName()),
				Namespace: defaultCleanupStateNamespace,
				Labels: map[string]string{
					appNameLabelKey:   cleanupStateAppName,
					ownerKindLabelKey: strings.TrimSpace(typed.GetKind()),
				},
			},
			Data: map[string][]byte{cleanupHandleDataKey: []byte(raw)},
		}
	default:
		return nil
	}
}
