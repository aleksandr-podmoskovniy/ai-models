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
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNormalizeRuntimeOptionsDefaultsImagePullPolicy(t *testing.T) {
	t.Parallel()

	options := NormalizeRuntimeOptions(RuntimeOptions{})
	if options.ImagePullPolicy != corev1.PullIfNotPresent {
		t.Fatalf("unexpected pull policy %q", options.ImagePullPolicy)
	}
	if got, want := options.WorkVolume.Type, WorkVolumeTypeEmptyDir; got != want {
		t.Fatalf("unexpected work volume type %q", got)
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

			options := withValidRuntimeOptions(RuntimeOptions{})
			switch tc.name {
			case "missing namespace":
				options.Namespace = ""
			case "missing image":
				options.Image = ""
			case "missing service account":
				options.ServiceAccountName = ""
			case "missing repository prefix":
				options.OCIRepositoryPrefix = ""
			case "missing registry secret":
				options.OCIRegistrySecretName = ""
			}

			err := ValidateRuntimeOptions("publication runtime", options)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("unexpected error %q", got)
			}
		})
	}
}

func TestValidateRuntimeOptionsRejectsMissingBoundedWorkVolumeSettings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*RuntimeOptions)
		wantErr string
	}{
		{
			name: "missing emptydir size limit",
			mutate: func(options *RuntimeOptions) {
				options.WorkVolume.EmptyDirSizeLimit = resource.Quantity{}
			},
			wantErr: "emptyDir sizeLimit",
		},
		{
			name: "missing pvc claim name",
			mutate: func(options *RuntimeOptions) {
				options.WorkVolume.Type = WorkVolumeTypePersistentVolumeClaim
				options.WorkVolume.PersistentVolumeClaimName = ""
			},
			wantErr: "persistentVolumeClaim name",
		},
		{
			name: "missing ephemeral storage request",
			mutate: func(options *RuntimeOptions) {
				delete(options.Resources.Requests, corev1.ResourceEphemeralStorage)
			},
			wantErr: "requests.ephemeral-storage",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := withValidRuntimeOptions(RuntimeOptions{})
			tc.mutate(&options)

			err := ValidateRuntimeOptions("publication runtime", options)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if got := err.Error(); !strings.Contains(got, tc.wantErr) {
				t.Fatalf("unexpected error %q", got)
			}
		})
	}
}

func withValidRuntimeOptions(options RuntimeOptions) RuntimeOptions {
	if strings.TrimSpace(options.Namespace) == "" {
		options.Namespace = "d8-ai-models"
	}
	if strings.TrimSpace(options.Image) == "" {
		options.Image = "controller-runtime:latest"
	}
	if strings.TrimSpace(options.ServiceAccountName) == "" {
		options.ServiceAccountName = "ai-models-controller"
	}
	if strings.TrimSpace(options.OCIRepositoryPrefix) == "" {
		options.OCIRepositoryPrefix = "registry.internal.local/ai-models"
	}
	if strings.TrimSpace(options.OCIRegistrySecretName) == "" {
		options.OCIRegistrySecretName = "ai-models-dmcr-auth-write"
	}
	if strings.TrimSpace(string(options.WorkVolume.Type)) == "" {
		options.WorkVolume.Type = WorkVolumeTypeEmptyDir
	}
	if options.WorkVolume.EmptyDirSizeLimit.Sign() <= 0 {
		options.WorkVolume.EmptyDirSizeLimit = resource.MustParse("2Ti")
	}
	if strings.TrimSpace(options.WorkVolume.PersistentVolumeClaimName) == "" {
		options.WorkVolume.PersistentVolumeClaimName = "ai-models-publication-work"
	}
	if options.Resources.Requests == nil {
		options.Resources.Requests = corev1.ResourceList{}
	}
	if options.Resources.Limits == nil {
		options.Resources.Limits = corev1.ResourceList{}
	}
	if _, ok := options.Resources.Requests[corev1.ResourceCPU]; !ok {
		options.Resources.Requests[corev1.ResourceCPU] = resource.MustParse("1")
	}
	if _, ok := options.Resources.Requests[corev1.ResourceMemory]; !ok {
		options.Resources.Requests[corev1.ResourceMemory] = resource.MustParse("8Gi")
	}
	if _, ok := options.Resources.Requests[corev1.ResourceEphemeralStorage]; !ok {
		options.Resources.Requests[corev1.ResourceEphemeralStorage] = resource.MustParse("2Ti")
	}
	if _, ok := options.Resources.Limits[corev1.ResourceCPU]; !ok {
		options.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("4")
	}
	if _, ok := options.Resources.Limits[corev1.ResourceMemory]; !ok {
		options.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("16Gi")
	}
	if _, ok := options.Resources.Limits[corev1.ResourceEphemeralStorage]; !ok {
		options.Resources.Limits[corev1.ResourceEphemeralStorage] = resource.MustParse("2Ti")
	}
	return options
}
