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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldRunGarbageCollection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		secret corev1.Secret
		want   bool
	}{
		{
			name: "queued request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						RequestQueuedAtAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
		{
			name: "pending request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						switchAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: true,
		},
		{
			name: "completed request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						phaseAnnotationKey:  phaseDone,
						switchAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
		{
			name: "non request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						switchAnnotationKey: "2026-04-10T00:00:00Z",
					},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldRunGarbageCollection(test.secret); got != test.want {
				t.Fatalf("shouldRunGarbageCollection() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRequestClassificationPrecedence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		labels      map[string]string
		annotations map[string]string
		want        requestState
	}{
		{
			name:        "non request is ignored even with switch",
			annotations: map[string]string{switchAnnotationKey: "2026-04-10T00:00:00Z"},
			want:        requestStateIgnored,
		},
		{
			name:   "done wins over switch",
			labels: map[string]string{RequestLabelKey: RequestLabelValue},
			annotations: map[string]string{
				phaseAnnotationKey:  phaseDone,
				switchAnnotationKey: "2026-04-10T00:00:00Z",
			},
			want: requestStateCompleted,
		},
		{
			name:   "switch wins over requested at",
			labels: map[string]string{RequestLabelKey: RequestLabelValue},
			annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-10T00:00:00Z",
				switchAnnotationKey:          "2026-04-10T00:00:00Z",
			},
			want: requestStateActive,
		},
		{
			name:        "phase queued alone is not lifecycle truth",
			labels:      map[string]string{RequestLabelKey: RequestLabelValue},
			annotations: map[string]string{phaseAnnotationKey: phaseQueued},
			want:        requestStateIgnored,
		},
		{
			name:        "phase armed alone is not lifecycle truth",
			labels:      map[string]string{RequestLabelKey: RequestLabelValue},
			annotations: map[string]string{phaseAnnotationKey: phaseArmed},
			want:        requestStateIgnored,
		},
		{
			name:   "requested at selects queued",
			labels: map[string]string{RequestLabelKey: RequestLabelValue},
			annotations: map[string]string{
				RequestQueuedAtAnnotationKey: "2026-04-10T00:00:00Z",
				phaseAnnotationKey:           phaseQueued,
			},
			want: requestStateQueued,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := classifyRequest(corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Labels:      test.labels,
				Annotations: test.annotations,
			}})
			if got != test.want {
				t.Fatalf("classifyRequest() = %s, want %s", requestStateName(got), requestStateName(test.want))
			}
		})
	}
}

func TestHasPendingRequestUsesQueuedAndActiveOnly(t *testing.T) {
	t.Parallel()

	done := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels:      map[string]string{RequestLabelKey: RequestLabelValue},
		Annotations: map[string]string{phaseAnnotationKey: phaseDone},
	}}
	ignored := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels:      map[string]string{RequestLabelKey: RequestLabelValue},
		Annotations: map[string]string{phaseAnnotationKey: phaseArmed},
	}}
	queued := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels:      map[string]string{RequestLabelKey: RequestLabelValue},
		Annotations: map[string]string{RequestQueuedAtAnnotationKey: "2026-04-10T00:00:00Z"},
	}}
	active := corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels:      map[string]string{RequestLabelKey: RequestLabelValue},
		Annotations: map[string]string{switchAnnotationKey: "2026-04-10T00:00:00Z"},
	}}

	if hasPendingRequest([]corev1.Secret{done, ignored}) {
		t.Fatal("hasPendingRequest(done, ignored) = true, want false")
	}
	if !hasPendingRequest([]corev1.Secret{done, queued}) {
		t.Fatal("hasPendingRequest(done, queued) = false, want true")
	}
	if !hasPendingRequest([]corev1.Secret{done, active}) {
		t.Fatal("hasPendingRequest(done, active) = false, want true")
	}
}

func TestShouldActivateGarbageCollection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		secrets         []corev1.Secret
		activationDelay time.Duration
		want            bool
	}{
		{
			name: "queued request older than activation delay arms gc",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{RequestLabelKey: RequestLabelValue},
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: now.Add(-11 * time.Minute).Format(time.RFC3339Nano),
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            true,
		},
		{
			name: "fresh queued request stays pending",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{RequestLabelKey: RequestLabelValue},
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: now.Add(-2 * time.Minute).Format(time.RFC3339Nano),
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            false,
		},
		{
			name: "invalid queued timestamp arms gc fail-open",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{RequestLabelKey: RequestLabelValue},
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: "broken",
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            true,
		},
		{
			name: "completed request stays ignored",
			secrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{RequestLabelKey: RequestLabelValue},
						Annotations: map[string]string{
							RequestQueuedAtAnnotationKey: now.Add(-11 * time.Minute).Format(time.RFC3339Nano),
							phaseAnnotationKey:           phaseDone,
						},
					},
				},
			},
			activationDelay: 10 * time.Minute,
			want:            false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldActivateGarbageCollection(test.secrets, now, test.activationDelay); got != test.want {
				t.Fatalf("shouldActivateGarbageCollection() = %t, want %t", got, test.want)
			}
		})
	}
}

func requestStateName(state requestState) string {
	switch state {
	case requestStateIgnored:
		return "ignored"
	case requestStateQueued:
		return "queued"
	case requestStateActive:
		return "active"
	case requestStateCompleted:
		return "completed"
	default:
		return "unknown"
	}
}
