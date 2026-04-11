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

package auditevent

import (
	"bytes"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"log/slog"
)

func TestRecorderMirrorsAuditEventToLogger(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(1)
	var buffer bytes.Buffer

	recorder, err := New(fakeRecorder, slog.New(slog.NewTextHandler(&buffer, nil)).With("component", "controller"))
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	object := &modelsv1alpha1.Model{
		TypeMeta: metav1.TypeMeta{
			Kind:       modelsv1alpha1.ModelKind,
			APIVersion: modelsv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
		},
	}

	recorder.Record(object, auditsink.Record{
		Type:    corev1.EventTypeNormal,
		Reason:  "PublicationSucceeded",
		Message: "controller published artifact oci://demo",
	})

	select {
	case event := <-fakeRecorder.Events:
		if !strings.Contains(event, "PublicationSucceeded") {
			t.Fatalf("expected fake recorder event to contain reason, got %q", event)
		}
	default:
		t.Fatal("expected fake recorder event")
	}

	output := buffer.String()
	for _, expected := range []string{
		"component=controller",
		"kind=Model",
		"namespace=team-a",
		"name=demo",
		"reason=PublicationSucceeded",
		"controller published artifact oci://demo",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected logger output to contain %q, got %q", expected, output)
		}
	}
}
