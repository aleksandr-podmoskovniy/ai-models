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
	"context"
	"time"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	publicationports "github.com/deckhouse/ai-models/controller/internal/ports/publishop"
	uploadstagingports "github.com/deckhouse/ai-models/controller/internal/ports/uploadstaging"
	publication "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"k8s.io/apimachinery/pkg/types"
)

func testUploadRequest() publicationports.Request {
	return publicationports.Request{
		Owner: publicationports.Owner{
			Kind:      modelsv1alpha1.ModelKind,
			Name:      "deepseek-r1-upload",
			Namespace: "team-a",
			UID:       types.UID("1111-2224"),
		},
		Identity: publication.Identity{
			Scope:     publication.ScopeNamespaced,
			Namespace: "team-a",
			Name:      "deepseek-r1-upload",
		},
		Spec: modelsv1alpha1.ModelSpec{
			Source: modelsv1alpha1.ModelSourceSpec{
				Upload: &modelsv1alpha1.UploadModelSource{},
			},
		},
	}
}

func testUploadOptions() Options {
	return Options{
		Runtime: RuntimeOptions{
			Namespace:           "d8-ai-models",
			OCIRepositoryPrefix: "registry.internal.local/ai-models",
		},
		Gateway: GatewayOptions{
			ServiceName: "ai-models-controller",
			PublicHost:  "ai-models.example.com",
		},
		TokenTTL: 15 * time.Minute,
	}
}

type fakeMultipartStager struct {
	listMultipartUploadParts func(context.Context, uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error)
}

func (f *fakeMultipartStager) StartMultipartUpload(context.Context, uploadstagingports.StartMultipartUploadInput) (uploadstagingports.StartMultipartUploadOutput, error) {
	return uploadstagingports.StartMultipartUploadOutput{}, nil
}

func (f *fakeMultipartStager) PresignUploadPart(context.Context, uploadstagingports.PresignUploadPartInput) (uploadstagingports.PresignUploadPartOutput, error) {
	return uploadstagingports.PresignUploadPartOutput{}, nil
}

func (f *fakeMultipartStager) ListMultipartUploadParts(ctx context.Context, input uploadstagingports.ListMultipartUploadPartsInput) ([]uploadstagingports.UploadedPart, error) {
	if f != nil && f.listMultipartUploadParts != nil {
		return f.listMultipartUploadParts(ctx, input)
	}
	return nil, nil
}

func (f *fakeMultipartStager) CompleteMultipartUpload(context.Context, uploadstagingports.CompleteMultipartUploadInput) error {
	return nil
}

func (f *fakeMultipartStager) AbortMultipartUpload(context.Context, uploadstagingports.AbortMultipartUploadInput) error {
	return nil
}

func (f *fakeMultipartStager) Stat(context.Context, uploadstagingports.StatInput) (uploadstagingports.ObjectStat, error) {
	return uploadstagingports.ObjectStat{}, nil
}
