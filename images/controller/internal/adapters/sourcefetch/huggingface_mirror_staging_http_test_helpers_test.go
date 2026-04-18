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
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (f *fakeMirrorUploadStaging) handleUploadPart(writer http.ResponseWriter, request *http.Request) {
	partNumber, uploadID, err := parseUploadPartPath(request.URL.Path)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	upload, found := f.uploads[uploadID]
	if !found {
		http.NotFound(writer, request)
		return
	}
	payload, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	upload.parts[partNumber] = payload
	writer.Header().Set("ETag", `"etag-`+strconv.Itoa(int(partNumber))+`"`)
	writer.WriteHeader(http.StatusOK)
}

func parseUploadPartPath(rawPath string) (int32, string, error) {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	if len(parts) != 3 || parts[0] != "multipart" {
		return 0, "", os.ErrNotExist
	}
	partNumber, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return 0, "", err
	}
	return int32(partNumber), parts[1], nil
}
