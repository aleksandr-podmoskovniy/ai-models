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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
)

func TestSourceMirrorRawProvenance(t *testing.T) {
	t.Parallel()

	provenance := sourceMirrorRawProvenance(Options{
		RawStageBucket: "artifacts",
	}, &sourcefetch.SourceMirrorSnapshot{
		CleanupPrefix: "raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef",
		ObjectCount:   7,
		SizeBytes:     1024,
	})

	if got, want := provenance.RawURI, "s3://artifacts/raw/1111-2222/source-url/.mirror/huggingface/google/gemma-4-E2B-it/deadbeef"; got != want {
		t.Fatalf("unexpected raw URI %q", got)
	}
	if got, want := provenance.RawObjectCount, int64(7); got != want {
		t.Fatalf("unexpected raw object count %d", got)
	}
	if got, want := provenance.RawSizeBytes, int64(1024); got != want {
		t.Fatalf("unexpected raw size bytes %d", got)
	}
}
