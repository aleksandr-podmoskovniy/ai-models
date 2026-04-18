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

package publishworker

import (
	"context"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
)

func TestFetchRemotePerformsSingleStreamingOnlyAttempt(t *testing.T) {
	previousFetchRemoteModelFunc := fetchRemoteModelFunc
	t.Cleanup(func() {
		fetchRemoteModelFunc = previousFetchRemoteModelFunc
	})

	calls := 0
	fetchRemoteModelFunc = func(context.Context, sourcefetch.RemoteOptions) (sourcefetch.RemoteResult, error) {
		calls++
		return sourcefetch.RemoteResult{
			InputFormat: modelsv1alpha1.ModelInputFormatSafetensors,
		}, nil
	}

	_, err := fetchRemote(context.Background(), Options{
		SourceType: modelsv1alpha1.ModelSourceTypeHuggingFace,
		HFModelID:  "owner/model",
		Revision:   "main",
	})
	if err != nil {
		t.Fatalf("fetchRemote() error = %v", err)
	}
	if got, want := calls, 1; got != want {
		t.Fatalf("unexpected fetch attempt count %d", got)
	}
}
