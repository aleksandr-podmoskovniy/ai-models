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

package oci

import (
	"context"
	"fmt"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type preparedPublishLayer struct {
	Layer      modelpackports.PublishLayer
	Plan       publishLayerDescriptor
	Descriptor *publishLayerDescriptor
	SizeBytes  int64
}

func preparePublishLayerUploads(
	ctx context.Context,
	layers []modelpackports.PublishLayer,
	plans []publishLayerDescriptor,
) ([]preparedPublishLayer, int64, error) {
	if len(layers) != len(plans) {
		return nil, 0, fmt.Errorf("publish layer count %d does not match planned layer count %d", len(layers), len(plans))
	}

	prepared := make([]preparedPublishLayer, 0, len(layers))
	var totalSizeBytes int64
	for index, layer := range layers {
		item := preparedPublishLayer{
			Layer: layer,
			Plan:  plans[index],
		}

		if canOnePassUploadRawLayer(layer) {
			sizeBytes, err := rawPublishLayerSize(layer)
			if err != nil {
				return nil, 0, err
			}
			item.SizeBytes = sizeBytes
		} else {
			descriptor, err := describePublishLayer(ctx, layer)
			if err != nil {
				return nil, 0, err
			}
			item.Descriptor = &descriptor
			item.SizeBytes = descriptor.Size
		}

		totalSizeBytes += item.SizeBytes
		prepared = append(prepared, item)
	}

	return prepared, totalSizeBytes, nil
}
