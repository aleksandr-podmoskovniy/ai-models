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
	"bytes"
	"testing"
)

func TestStageRawObjectBuildsHandleAndUploadsPayload(t *testing.T) {
	t.Parallel()

	staging := newFakeUploadStaging()
	handle, err := stageRawObject(
		t.Context(),
		RawStageOptions{
			Bucket:    "artifacts",
			KeyPrefix: "raw/1111-2222/source-url",
			Client:    staging,
		},
		"model/config.json",
		"config.json",
		-1,
		"application/json",
		bytes.NewBufferString(`{"model":"deepseek"}`),
	)
	if err != nil {
		t.Fatalf("stageRawObject() error = %v", err)
	}
	if got, want := handle.Key, "raw/1111-2222/source-url/model/config.json"; got != want {
		t.Fatalf("unexpected key %q", got)
	}
	if got, want := handle.FileName, "config.json"; got != want {
		t.Fatalf("unexpected file name %q", got)
	}
	if got := handle.SizeBytes; got != 0 {
		t.Fatalf("unexpected size bytes %d", got)
	}
	if got, want := string(staging.objects["artifacts/raw/1111-2222/source-url/model/config.json"]), `{"model":"deepseek"}`; got != want {
		t.Fatalf("unexpected uploaded payload %q", got)
	}
}

func TestStageRawObjectRejectsMissingRelativePath(t *testing.T) {
	t.Parallel()

	_, err := stageRawObject(
		t.Context(),
		RawStageOptions{
			Bucket:    "artifacts",
			KeyPrefix: "raw/1111-2222/source-url",
			Client:    newFakeUploadStaging(),
		},
		"",
		"",
		1,
		"application/octet-stream",
		bytes.NewBufferString("payload"),
	)
	if err == nil {
		t.Fatal("expected error")
	}
}
