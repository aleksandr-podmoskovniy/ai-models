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

package sourceworker

import (
	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
)

func testOperationRequest() publicationports.Request {
	return publicationports.Request{
		Owner: publicationports.Owner{
			UID:       types.UID("1111-2222"),
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1",
			Namespace: "team-a",
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1",
		},
		Spec: modelsv1alpha1.ModelSpec{
			Source: modelsv1alpha1.ModelSourceSpec{
				URL: "https://huggingface.co/deepseek-ai/DeepSeek-R1",
			},
		},
	}
}

func testOptions() Options {
	return Options{
		RuntimeOptions: RuntimeOptions{
			Namespace:               "d8-ai-models",
			Image:                   "backend:latest",
			ImagePullSecretName:     "ai-models-module-registry",
			ServiceAccountName:      "ai-models-controller",
			OCIRepositoryPrefix:     "registry.internal.local/ai-models",
			OCIRegistrySecretName:   "ai-models-dmcr-auth-write",
			OCIDirectUploadEndpoint: "https://ai-models-dmcr.d8-ai-models.svc.cluster.local:5443",
			SourceAcquisition:       publicationports.SourceAcquisitionModeMirror,
			ObjectStorage: storageprojection.Options{
				Bucket:                "ai-models",
				EndpointURL:           "https://s3.example.com",
				Region:                "us-east-1",
				UsePathStyle:          true,
				CredentialsSecretName: "ai-models-artifacts",
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("1"),
					corev1.ResourceMemory:           resource.MustParse("8Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Ti"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:              resource.MustParse("4"),
					corev1.ResourceMemory:           resource.MustParse("16Gi"),
					corev1.ResourceEphemeralStorage: resource.MustParse("2Ti"),
				},
			},
		},
		LogFormat:            "json",
		LogLevel:             "debug",
		MaxConcurrentWorkers: 1,
	}
}
