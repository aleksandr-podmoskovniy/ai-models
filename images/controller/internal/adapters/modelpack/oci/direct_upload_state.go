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
	"errors"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

type directUploadCheckpoint struct {
	store modelpackports.DirectUploadStateStore
	state modelpackports.DirectUploadState
}

func loadDirectUploadCheckpoint(
	ctx context.Context,
	input modelpackports.PublishInput,
) (*directUploadCheckpoint, error) {
	if input.DirectUploadState == nil {
		return nil, nil
	}
	state, found, err := input.DirectUploadState.Load(ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return &directUploadCheckpoint{
		store: input.DirectUploadState,
		state: state,
	}, nil
}

func (c *directUploadCheckpoint) completedLayer(plan publishLayerDescriptor) (publishLayerDescriptor, bool, error) {
	if c == nil {
		return publishLayerDescriptor{}, false, nil
	}
	key := directUploadLayerKey(plan)
	for _, layer := range c.state.CompletedLayers {
		if layer.Key != key {
			continue
		}
		if layer.TargetPath != plan.TargetPath || layer.MediaType != plan.MediaType {
			return publishLayerDescriptor{}, false, errors.New("direct upload completed layer checkpoint does not match current plan")
		}
		return descriptorFromStateLayer(layer), true, nil
	}
	return publishLayerDescriptor{}, false, nil
}

func (c *directUploadCheckpoint) currentLayer(plan publishLayerDescriptor) *modelpackports.DirectUploadCurrentLayer {
	if c == nil || c.state.Phase != modelpackports.DirectUploadStatePhaseRunning || c.state.CurrentLayer == nil {
		return nil
	}
	if c.state.CurrentLayer.Key != directUploadLayerKey(plan) {
		return nil
	}
	current := *c.state.CurrentLayer
	return &current
}

func (c *directUploadCheckpoint) saveRunningLayer(
	ctx context.Context,
	current modelpackports.DirectUploadCurrentLayer,
	stage modelpackports.DirectUploadStateStage,
) error {
	if c == nil {
		return nil
	}
	c.state.Phase = modelpackports.DirectUploadStatePhaseRunning
	c.state.Stage = stage
	c.state.FailureMessage = ""
	c.state.CurrentLayer = &current
	return c.store.Save(ctx, c.state)
}

func (c *directUploadCheckpoint) markSealing(
	ctx context.Context,
	current modelpackports.DirectUploadCurrentLayer,
) error {
	if c == nil {
		return nil
	}
	c.state.Phase = modelpackports.DirectUploadStatePhaseRunning
	c.state.Stage = modelpackports.DirectUploadStateStageSealing
	c.state.FailureMessage = ""
	c.state.CurrentLayer = &current
	return c.store.Save(ctx, c.state)
}

func (c *directUploadCheckpoint) markLayerCompleted(
	ctx context.Context,
	descriptor publishLayerDescriptor,
) error {
	if c == nil {
		return nil
	}
	key := directUploadLayerKey(descriptor)
	completed := make([]modelpackports.DirectUploadLayerDescriptor, 0, len(c.state.CompletedLayers)+1)
	replaced := false
	for _, layer := range c.state.CompletedLayers {
		if layer.Key == key {
			completed = append(completed, stateLayerFromDescriptor(descriptor))
			replaced = true
			continue
		}
		completed = append(completed, layer)
	}
	if !replaced {
		completed = append(completed, stateLayerFromDescriptor(descriptor))
	}
	c.state.Phase = modelpackports.DirectUploadStatePhaseRunning
	c.state.Stage = modelpackports.DirectUploadStateStageCommitted
	c.state.FailureMessage = ""
	c.state.CurrentLayer = nil
	c.state.CompletedLayers = completed
	return c.store.Save(ctx, c.state)
}

func directUploadLayerKey(plan publishLayerDescriptor) string {
	return plan.TargetPath + "|" + plan.MediaType
}

func stateLayerFromDescriptor(descriptor publishLayerDescriptor) modelpackports.DirectUploadLayerDescriptor {
	return modelpackports.DirectUploadLayerDescriptor{
		Key:         directUploadLayerKey(descriptor),
		Digest:      descriptor.Digest,
		DiffID:      descriptor.DiffID,
		SizeBytes:   descriptor.Size,
		MediaType:   descriptor.MediaType,
		TargetPath:  descriptor.TargetPath,
		Base:        descriptor.Base,
		Format:      descriptor.Format,
		Compression: descriptor.Compression,
	}
}

func descriptorFromStateLayer(layer modelpackports.DirectUploadLayerDescriptor) publishLayerDescriptor {
	return publishLayerDescriptor{
		Digest:      layer.Digest,
		DiffID:      layer.DiffID,
		Size:        layer.SizeBytes,
		MediaType:   layer.MediaType,
		TargetPath:  layer.TargetPath,
		Base:        layer.Base,
		Format:      layer.Format,
		Compression: layer.Compression,
	}
}
