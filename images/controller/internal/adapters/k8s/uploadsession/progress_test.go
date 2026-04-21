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
	"testing"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
)

func TestLocalUploadProgress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		session *uploadsessionstate.Session
		want    string
	}{
		{
			name: "issued session starts at zero",
			session: &uploadsessionstate.Session{
				Phase: uploadsessionstate.PhaseIssued,
			},
			want: "0%",
		},
		{
			name: "uploading session reports multipart percentage",
			session: &uploadsessionstate.Session{
				Phase:             uploadsessionstate.PhaseUploading,
				ExpectedSizeBytes: 200,
				Multipart: &uploadsessionruntime.SessionState{
					UploadedParts: []uploadsessionruntime.UploadedPart{
						{PartNumber: 1, ETag: "etag-1", SizeBytes: 50},
						{PartNumber: 2, ETag: "etag-2", SizeBytes: 50},
					},
				},
			},
			want: "50%",
		},
		{
			name: "uploading session caps at one hundred",
			session: &uploadsessionstate.Session{
				Phase:             uploadsessionstate.PhaseUploading,
				ExpectedSizeBytes: 100,
				Multipart: &uploadsessionruntime.SessionState{
					UploadedParts: []uploadsessionruntime.UploadedPart{
						{PartNumber: 1, ETag: "etag-1", SizeBytes: 120},
					},
				},
			},
			want: "100%",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := localUploadProgress(tc.session); got != tc.want {
				t.Fatalf("localUploadProgress() = %q, want %q", got, tc.want)
			}
		})
	}
}
