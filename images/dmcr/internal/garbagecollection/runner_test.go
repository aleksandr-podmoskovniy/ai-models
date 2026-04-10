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
			name: "done request secret",
			secret: corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{RequestLabelKey: RequestLabelValue},
					Annotations: map[string]string{
						doneAnnotationKey: "2026-04-10T00:00:00Z",
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
