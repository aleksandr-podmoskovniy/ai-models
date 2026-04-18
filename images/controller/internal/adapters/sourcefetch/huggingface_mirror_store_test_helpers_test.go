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

package sourcefetch

import (
	"context"

	sourcemirrorports "github.com/deckhouse/ai-models/controller/internal/ports/sourcemirror"
)

type fakeSourceMirrorStore struct {
	manifest sourcemirrorports.SnapshotManifest
	state    sourcemirrorports.SnapshotState
}

func (f *fakeSourceMirrorStore) SaveManifest(_ context.Context, manifest sourcemirrorports.SnapshotManifest) error {
	f.manifest = manifest
	return nil
}

func (f *fakeSourceMirrorStore) LoadManifest(context.Context, sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotManifest, error) {
	return f.manifest, nil
}

func (f *fakeSourceMirrorStore) SaveState(_ context.Context, state sourcemirrorports.SnapshotState) error {
	f.state = state
	return nil
}

func (f *fakeSourceMirrorStore) LoadState(context.Context, sourcemirrorports.SnapshotLocator) (sourcemirrorports.SnapshotState, error) {
	if f.state.Locator == (sourcemirrorports.SnapshotLocator{}) {
		return sourcemirrorports.SnapshotState{}, sourcemirrorports.ErrSnapshotNotFound
	}
	return f.state, nil
}

func (f *fakeSourceMirrorStore) DeleteSnapshot(context.Context, sourcemirrorports.SnapshotLocator) error {
	f.manifest = sourcemirrorports.SnapshotManifest{}
	f.state = sourcemirrorports.SnapshotState{}
	return nil
}
