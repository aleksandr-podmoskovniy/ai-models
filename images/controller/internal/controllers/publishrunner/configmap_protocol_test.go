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

package publishrunner

import (
	"encoding/json"
	"testing"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRequiredPayloadDecodersFailClosed(t *testing.T) {
	t.Parallel()

	decodeRequest := func(configMap *corev1.ConfigMap) error {
		_, err := RequestFromConfigMap(configMap)
		return err
	}
	decodeResult := func(configMap *corev1.ConfigMap) error {
		_, err := ResultFromConfigMap(configMap)
		return err
	}

	cases := []struct {
		name   string
		decode func(*corev1.ConfigMap) error
		input  *corev1.ConfigMap
	}{
		{name: "request nil", decode: decodeRequest},
		{name: "request missing", decode: decodeRequest, input: &corev1.ConfigMap{}},
		{name: "request whitespace", decode: decodeRequest, input: dataConfigMap(requestDataKey, "   ")},
		{name: "request malformed", decode: decodeRequest, input: dataConfigMap(requestDataKey, "{not-json")},
		{name: "request invalid", decode: decodeRequest, input: dataConfigMap(requestDataKey, `{"owner":{"kind":"Model","name":"deepseek-r1"}}`)},
		{name: "result nil", decode: decodeResult},
		{name: "result missing", decode: decodeResult, input: &corev1.ConfigMap{}},
		{name: "result whitespace", decode: decodeResult, input: dataConfigMap(resultDataKey, "   ")},
		{name: "result malformed", decode: decodeResult, input: dataConfigMap(resultDataKey, "{not-json")},
		{name: "result invalid", decode: decodeResult, input: dataConfigMap(resultDataKey, `{"snapshot":{},"cleanupHandle":{"kind":"BackendArtifact","backend":{}}}`)},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.decode(tc.input); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestAccessorsTrimAndDefault(t *testing.T) {
	t.Parallel()

	if IsManagedConfigMap(nil) {
		t.Fatal("nil configmap must not be managed")
	}
	if WorkerResultFromConfigMap(nil) != "" {
		t.Fatal("expected empty worker result for nil configmap")
	}
	if WorkerFailureFromConfigMap(nil) != "" {
		t.Fatal("expected empty worker failure for nil configmap")
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{managedLabelKey: managedLabelValue},
			Annotations: map[string]string{
				messageAnnotationKey: "  waiting  ",
				workerAnnotationKey:  "  worker-a  ",
			},
		},
		Data: map[string]string{
			workerResultDataKey:  "  result  ",
			workerFailureDataKey: "  failed  ",
		},
	}

	if !IsManagedConfigMap(configMap) {
		t.Fatal("expected managed configmap")
	}
	if got, want := WorkerResultFromConfigMap(configMap), "result"; got != want {
		t.Fatalf("unexpected worker result %q", got)
	}
	if got, want := WorkerFailureFromConfigMap(configMap), "failed"; got != want {
		t.Fatalf("unexpected worker failure %q", got)
	}

	status := StatusFromConfigMap(configMap)
	if got, want := status.Phase, PhasePending; got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := status.Message, "waiting"; got != want {
		t.Fatalf("unexpected message %q", got)
	}
	if got, want := status.WorkerName, "worker-a"; got != want {
		t.Fatalf("unexpected worker %q", got)
	}
}

func TestUploadStatusFromConfigMap(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    *corev1.ConfigMap
		wantNil  bool
		wantErr  bool
		wantRepo string
		wantCmd  string
	}{
		{name: "missing returns nil", input: &corev1.ConfigMap{}, wantNil: true},
		{name: "whitespace returns nil", input: dataConfigMap(uploadDataKey, "   "), wantNil: true},
		{name: "malformed fails", input: dataConfigMap(uploadDataKey, "{not-json}"), wantErr: true},
		{name: "invalid missing repository fails", input: dataConfigMap(uploadDataKey, `{"command":"curl","expiresAt":"2026-04-03T10:00:00Z"}`), wantErr: true},
		{name: "valid payload", input: dataConfigMap(uploadDataKey, `{"command":"curl","repository":"repo","expiresAt":"2026-04-03T10:00:00Z"}`), wantRepo: "repo", wantCmd: "curl"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			status, err := UploadStatusFromConfigMap(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
			}
			if tc.wantNil {
				if status != nil {
					t.Fatalf("expected nil status, got %#v", status)
				}
				return
			}
			if status == nil {
				t.Fatal("expected upload status")
			}
			if got, want := status.Repository, tc.wantRepo; got != want {
				t.Fatalf("unexpected repository %q", got)
			}
			if got, want := status.Command, tc.wantCmd; got != want {
				t.Fatalf("unexpected command %q", got)
			}
		})
	}
}

func TestValidatePersistedStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   *corev1.ConfigMap
		status  publicationports.Status
		wantErr bool
	}{
		{name: "pending is accepted", input: &corev1.ConfigMap{}, status: publicationports.Status{Phase: publicationports.PhasePending}},
		{name: "failed is accepted", input: &corev1.ConfigMap{}, status: publicationports.Status{Phase: publicationports.PhaseFailed}},
		{name: "running with malformed upload fails", input: dataConfigMap(uploadDataKey, `{}`), status: publicationports.Status{Phase: publicationports.PhaseRunning}, wantErr: true},
		{name: "succeeded without result fails", input: &corev1.ConfigMap{}, status: publicationports.Status{Phase: publicationports.PhaseSucceeded}, wantErr: true},
		{name: "succeeded with valid result passes", input: dataConfigMap(resultDataKey, mustMarshalResult(t)), status: publicationports.Status{Phase: publicationports.PhaseSucceeded}},
		{name: "unsupported phase fails", input: &corev1.ConfigMap{}, status: publicationports.Status{Phase: publicationports.Phase("Weird")}, wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validatePersistedStatus(tc.input, tc.status)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validatePersistedStatus() error = %v", err)
			}
		})
	}
}

func TestMutationHelpersFailClosed(t *testing.T) {
	t.Parallel()

	invalidUpload := modelsv1alpha1.ModelUploadStatus{}
	cases := []struct {
		name   string
		mutate func() error
	}{
		{name: "SetRunning nil configmap", mutate: func() error { return SetRunning(nil, "worker-a") }},
		{name: "SetRunning empty worker", mutate: func() error { return SetRunning(&corev1.ConfigMap{}, " ") }},
		{name: "SetUploadReady nil configmap", mutate: func() error { return SetUploadReady(nil, invalidUpload) }},
		{name: "SetUploadReady invalid upload", mutate: func() error { return SetUploadReady(&corev1.ConfigMap{}, invalidUpload) }},
		{name: "SetFailed nil configmap", mutate: func() error { return SetFailed(nil, "boom") }},
		{name: "SetSucceeded nil configmap", mutate: func() error { return SetSucceeded(nil, sampleResult()) }},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.mutate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSetFailedClearsTransientPayloads(t *testing.T) {
	t.Parallel()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{workerAnnotationKey: "worker-a"},
		},
		Data: map[string]string{
			uploadDataKey: `{"command":"curl"}`,
			resultDataKey: `{"stale":"result"}`,
		},
	}

	if err := SetFailed(configMap, "  failed import  "); err != nil {
		t.Fatalf("SetFailed() error = %v", err)
	}
	if got, want := configMap.Annotations[phaseAnnotationKey], string(PhaseFailed); got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if got, want := configMap.Annotations[messageAnnotationKey], "failed import"; got != want {
		t.Fatalf("unexpected message %q", got)
	}
	if _, found := configMap.Annotations[workerAnnotationKey]; found {
		t.Fatal("worker annotation must be cleared on failure")
	}
	if _, found := configMap.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared on failure")
	}
	if _, found := configMap.Data[resultDataKey]; found {
		t.Fatal("result payload must be cleared on failure")
	}
}

func TestSetSucceededPersistsResultAndClearsTransientPayloads(t *testing.T) {
	t.Parallel()

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				messageAnnotationKey: "failed import",
				workerAnnotationKey:  "worker-a",
			},
		},
		Data: map[string]string{
			uploadDataKey: `{"command":"curl"}`,
		},
	}

	if err := SetSucceeded(configMap, sampleResult()); err != nil {
		t.Fatalf("SetSucceeded() error = %v", err)
	}
	if got, want := configMap.Annotations[phaseAnnotationKey], string(PhaseSucceeded); got != want {
		t.Fatalf("unexpected phase %q", got)
	}
	if _, found := configMap.Annotations[messageAnnotationKey]; found {
		t.Fatal("failure message must be cleared on success")
	}
	if _, found := configMap.Annotations[workerAnnotationKey]; found {
		t.Fatal("worker annotation must be cleared on success")
	}
	if _, found := configMap.Data[uploadDataKey]; found {
		t.Fatal("upload payload must be cleared on success")
	}
	if _, err := ResultFromConfigMap(configMap); err != nil {
		t.Fatalf("ResultFromConfigMap() error = %v", err)
	}
}

func TestSetUploadReadyPersistsValidatedPayload(t *testing.T) {
	t.Parallel()

	expiresAt := metav1.NewTime(time.Unix(1712345678, 0).UTC())
	configMap := &corev1.ConfigMap{}
	if err := SetUploadReady(configMap, modelsv1alpha1.ModelUploadStatus{
		ExpiresAt:  &expiresAt,
		Repository: "registry.internal.local/model:upload",
		Command:    "curl -T file",
	}); err != nil {
		t.Fatalf("SetUploadReady() error = %v", err)
	}

	status, err := UploadStatusFromConfigMap(configMap)
	if err != nil {
		t.Fatalf("UploadStatusFromConfigMap() error = %v", err)
	}
	if status == nil {
		t.Fatal("expected upload status")
	}
	if got, want := status.Command, "curl -T file"; got != want {
		t.Fatalf("unexpected command %q", got)
	}
}

func dataConfigMap(key, value string) *corev1.ConfigMap {
	return &corev1.ConfigMap{Data: map[string]string{key: value}}
}

func mustMarshalResult(t *testing.T) string {
	t.Helper()

	payload, err := json.Marshal(sampleResult())
	if err != nil {
		t.Fatalf("json.Marshal(sampleResult()) error = %v", err)
	}
	return string(payload)
}
