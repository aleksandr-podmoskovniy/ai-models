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
	"fmt"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/uploadsessionstate"
	uploadsessionruntime "github.com/deckhouse/ai-models/controller/internal/dataplane/uploadsession"
)

func localUploadProgress(session *uploadsessionstate.Session) string {
	if session == nil {
		return ""
	}

	switch session.Phase {
	case uploadsessionstate.PhaseIssued, uploadsessionstate.PhaseProbing:
		return "0%"
	case uploadsessionstate.PhaseUploading:
		return multipartUploadProgress(session.ExpectedSizeBytes, session.Multipart)
	default:
		return ""
	}
}

func multipartUploadProgress(expectedSizeBytes int64, state *uploadsessionruntime.SessionState) string {
	if expectedSizeBytes <= 0 || state == nil {
		return ""
	}

	var uploadedSizeBytes int64
	for _, part := range state.UploadedParts {
		if part.SizeBytes > 0 {
			uploadedSizeBytes += part.SizeBytes
		}
	}
	switch {
	case uploadedSizeBytes <= 0:
		return "0%"
	case uploadedSizeBytes >= expectedSizeBytes:
		return "100%"
	default:
		return fmt.Sprintf("%d%%", int((uploadedSizeBytes*100)/expectedSizeBytes))
	}
}
