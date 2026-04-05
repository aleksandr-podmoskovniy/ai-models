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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSessionTerminalStateHelpers(t *testing.T) {
	t.Parallel()

	if !IsComplete(&Session{Pod: &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}}) {
		t.Fatal("expected complete session")
	}
	if !IsFailed(&Session{Pod: &corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}}) {
		t.Fatal("expected failed session")
	}
}

func TestSessionFromResourcesFailsClosedOnEmptyToken(t *testing.T) {
	t.Parallel()

	_, err := sessionFromResources(
		&corev1.Pod{},
		&corev1.Service{},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"ai-models.deckhouse.io/upload-expires-at": time.Now().UTC().Format(time.RFC3339)},
		}},
	)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExpiresAtFromSecretFailsClosedOnMalformedValue(t *testing.T) {
	t.Parallel()

	_, err := expiresAtFromSecret(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"ai-models.deckhouse.io/upload-expires-at": "not-a-time"},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
