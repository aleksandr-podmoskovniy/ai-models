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

package workloadpod

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestNormalizeRuntimeOptionsDefaultsImagePullPolicy(t *testing.T) {
	t.Parallel()

	options := NormalizeRuntimeOptions(RuntimeOptions{})
	if options.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatalf("unexpected pull policy %q", options.ImagePullPolicy)
	}
}

func TestValidateRuntimeOptionsRejectsMissingFields(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		options RuntimeOptions
		wantErr string
	}{
		{
			name:    "missing namespace",
			options: RuntimeOptions{},
			wantErr: "namespace",
		},
		{
			name: "missing image",
			options: RuntimeOptions{
				Namespace: "d8-ai-models",
			},
			wantErr: "image",
		},
		{
			name: "missing service account",
			options: RuntimeOptions{
				Namespace: "d8-ai-models",
				Image:     "controller-runtime:latest",
			},
			wantErr: "serviceAccountName",
		},
		{
			name: "missing repository prefix",
			options: RuntimeOptions{
				Namespace:          "d8-ai-models",
				Image:              "controller-runtime:latest",
				ServiceAccountName: "ai-models-controller",
			},
			wantErr: "OCI repository prefix",
		},
		{
			name: "missing registry secret",
			options: RuntimeOptions{
				Namespace:           "d8-ai-models",
				Image:               "controller-runtime:latest",
				ServiceAccountName:  "ai-models-controller",
				OCIRepositoryPrefix: "registry.internal.local/ai-models",
			},
			wantErr: "OCI registry secret name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRuntimeOptions("publication runtime", tc.options)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("unexpected error %q", got)
			}
		})
	}
}
