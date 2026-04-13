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
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/adapters/sourcefetch"
	publicationdata "github.com/deckhouse/ai-models/controller/internal/publishedsnapshot"
	"github.com/deckhouse/ai-models/controller/internal/support/cleanuphandle"
)

func rawURI(bucket, key string) string {
	if strings.TrimSpace(bucket) == "" || strings.TrimSpace(key) == "" {
		return ""
	}
	return fmt.Sprintf("s3://%s/%s", strings.TrimSpace(bucket), strings.Trim(strings.TrimSpace(key), "/"))
}

func uploadRawProvenance(handle *cleanuphandle.UploadStagingHandle) publicationdata.SourceProvenance {
	if handle == nil {
		return publicationdata.SourceProvenance{}
	}
	return publicationdata.SourceProvenance{
		RawURI:         rawURI(handle.Bucket, handle.Key),
		RawObjectCount: 1,
		RawSizeBytes:   nonNegativeSize(handle.SizeBytes),
	}
}

func remoteRawProvenance(options Options, objects []cleanuphandle.UploadStagingHandle) publicationdata.SourceProvenance {
	provenance := publicationdata.SourceProvenance{}
	if strings.TrimSpace(options.RawStageBucket) != "" && strings.TrimSpace(options.RawStageKeyPrefix) != "" {
		provenance.RawURI = rawURI(options.RawStageBucket, options.RawStageKeyPrefix)
	}
	provenance.RawObjectCount = int64(len(objects))
	for _, object := range objects {
		provenance.RawSizeBytes += nonNegativeSize(object.SizeBytes)
	}
	return provenance
}

func sourceMirrorRawProvenance(options Options, sourceMirror *sourcefetch.SourceMirrorSnapshot) publicationdata.SourceProvenance {
	if sourceMirror == nil {
		return publicationdata.SourceProvenance{}
	}
	return publicationdata.SourceProvenance{
		RawURI:         rawURI(options.RawStageBucket, sourceMirror.CleanupPrefix),
		RawObjectCount: sourceMirror.ObjectCount,
		RawSizeBytes:   nonNegativeSize(sourceMirror.SizeBytes),
	}
}

func nonNegativeSize(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
