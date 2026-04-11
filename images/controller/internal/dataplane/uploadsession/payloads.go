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

type sessionInfoResponse struct {
	Mode                      string                 `json:"mode"`
	Phase                     string                 `json:"phase,omitempty"`
	ExpectedSizeBytes         int64                  `json:"expectedSizeBytes,omitempty"`
	DeclaredInputFormat       string                 `json:"declaredInputFormat,omitempty"`
	FailureMessage            string                 `json:"failureMessage,omitempty"`
	PartURLTTLSeconds         int64                  `json:"partURLTTLSeconds"`
	MinimumPartSizeBytes      int64                  `json:"minimumPartSizeBytes"`
	MaximumMultipartPartCount int                    `json:"maximumMultipartPartCount"`
	Probe                     *probeStatePayload     `json:"probe,omitempty"`
	Multipart                 *multipartStatePayload `json:"multipart,omitempty"`
}

type probeUploadRequest struct {
	FileName string `json:"fileName"`
	Chunk    []byte `json:"chunk"`
}

type probeUploadResponse struct {
	FileName            string `json:"fileName"`
	ResolvedInputFormat string `json:"resolvedInputFormat,omitempty"`
}

type probeStatePayload struct {
	FileName            string `json:"fileName"`
	ResolvedInputFormat string `json:"resolvedInputFormat,omitempty"`
}

type multipartStatePayload struct {
	UploadID      string                `json:"uploadID"`
	Key           string                `json:"key"`
	FileName      string                `json:"fileName"`
	UploadedParts []uploadedPartPayload `json:"uploadedParts,omitempty"`
}

type uploadedPartPayload struct {
	PartNumber int32  `json:"partNumber"`
	ETag       string `json:"etag"`
	SizeBytes  int64  `json:"sizeBytes,omitempty"`
}

type initUploadRequest struct {
	FileName string `json:"fileName"`
}

type initUploadResponse struct {
	UploadID string `json:"uploadID"`
	Key      string `json:"key"`
	FileName string `json:"fileName"`
}

type presignPartsRequest struct {
	PartNumbers []int32 `json:"partNumbers"`
}

type presignedPartPayload struct {
	PartNumber int32  `json:"partNumber"`
	URL        string `json:"url"`
}

type presignPartsResponse struct {
	UploadID string                 `json:"uploadID"`
	Parts    []presignedPartPayload `json:"parts"`
}

type completedPartRequest struct {
	PartNumber int32  `json:"partNumber"`
	ETag       string `json:"etag"`
}

type completeUploadRequest struct {
	Parts []completedPartRequest `json:"parts"`
}
