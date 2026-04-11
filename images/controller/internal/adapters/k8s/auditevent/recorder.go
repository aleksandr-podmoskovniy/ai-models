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
	"errors"
	"log/slog"
	"reflect"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/ports/auditsink"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Recorder struct {
	recorder record.EventRecorder
	logger   *slog.Logger
}

func New(recorder record.EventRecorder, logger *slog.Logger) (*Recorder, error) {
	if recorder == nil {
		return nil, errors.New("audit event recorder must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Recorder{recorder: recorder, logger: logger}, nil
}

func (r *Recorder) Record(object client.Object, event auditsink.Record) {
	if r == nil || r.recorder == nil || object == nil {
		return
	}
	if strings.TrimSpace(event.Type) == "" || strings.TrimSpace(event.Reason) == "" || strings.TrimSpace(event.Message) == "" {
		return
	}
	r.logRecord(object, event)
	r.recorder.Event(object, event.Type, event.Reason, event.Message)
}

func (r *Recorder) logRecord(object client.Object, event auditsink.Record) {
	if r == nil || r.logger == nil || object == nil {
		return
	}

	args := []any{
		slog.String("kind", objectKind(object)),
		slog.String("name", object.GetName()),
		slog.String("reason", event.Reason),
		slog.String("eventType", event.Type),
	}
	if namespace := strings.TrimSpace(object.GetNamespace()); namespace != "" {
		args = append(args, slog.String("namespace", namespace))
	}

	switch event.Type {
	case corev1.EventTypeWarning:
		r.logger.Warn(event.Message, args...)
	default:
		r.logger.Info(event.Message, args...)
	}
}

func objectKind(object client.Object) string {
	if object == nil {
		return ""
	}
	if kind := strings.TrimSpace(object.GetObjectKind().GroupVersionKind().Kind); kind != "" {
		return kind
	}
	value := reflect.Indirect(reflect.ValueOf(object))
	if !value.IsValid() {
		return ""
	}
	return value.Type().Name()
}
