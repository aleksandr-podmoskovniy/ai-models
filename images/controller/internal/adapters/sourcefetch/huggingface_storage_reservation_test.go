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
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestFetchRemoteModelHuggingFaceReservesDirectSizeBeforePublishPlanning(t *testing.T) {
	configPayload := []byte(`{"architectures":["LlamaForCausalLM"]}`)
	server := reservationTestServer(t, configPayload, []byte("tensor-payload"))
	defer server.Close()

	stubDefaultHuggingFaceInfo(t)
	withHuggingFaceBaseURL(t, server.URL)
	reservations := &fakeRemoteStorageReservation{}

	_, err := fetchTestHuggingFaceRemoteWithReservation(t, nil, reservations)
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}
	if got := len(reservations.requests); got != 1 {
		t.Fatalf("reservation count = %d, want 1", got)
	}
	request := reservations.requests[0]
	if request.SourceFetchMode != "direct" {
		t.Fatalf("reservation source fetch mode = %q, want direct", request.SourceFetchMode)
	}
	want, err := estimateHuggingFaceCanonicalArtifactBytes(reservationRemoteFiles(configPayload, []byte("tensor-payload")))
	if err != nil {
		t.Fatalf("estimateHuggingFaceCanonicalArtifactBytes() error = %v", err)
	}
	if got := request.SizeBytes; got != want {
		t.Fatalf("reservation size bytes = %d, want %d", got, want)
	}
}

func TestFetchRemoteModelHuggingFaceStopsWhenDirectReservationFails(t *testing.T) {
	server := reservationTestServer(t, []byte(`{"architectures":["LlamaForCausalLM"]}`), []byte("tensor-payload"))
	defer server.Close()

	stubDefaultHuggingFaceInfo(t)
	withHuggingFaceBaseURL(t, server.URL)
	reservations := &fakeRemoteStorageReservation{err: errors.New("no space")}

	_, err := fetchTestHuggingFaceRemoteWithReservation(t, nil, reservations)
	if err == nil {
		t.Fatal("FetchRemoteModel() error = nil, want reservation failure")
	}
}

func TestFetchRemoteModelHuggingFaceReservesMirrorDoubleOwnedBytesBeforeTransfer(t *testing.T) {
	configPayload := []byte(`{"architectures":["LlamaForCausalLM"]}`)
	modelPayload := []byte("tensor-payload")
	server := reservationTestServer(t, configPayload, modelPayload)
	defer server.Close()

	stubDefaultHuggingFaceInfo(t)
	withHuggingFaceBaseURL(t, server.URL)
	stubHuggingFaceProfileSummary(t, defaultHuggingFaceProfileSummary(), nil)

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	reservations := &fakeRemoteStorageReservation{}
	_, err := fetchTestHuggingFaceRemoteWithReservation(t, newTestSourceMirrorOptions(mirrorClient, mirrorStore), reservations)
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if got := len(reservations.requests); got != 1 {
		t.Fatalf("reservation count = %d, want 1", got)
	}
	request := reservations.requests[0]
	if request.SourceFetchMode != "mirror" {
		t.Fatalf("reservation source fetch mode = %q, want mirror", request.SourceFetchMode)
	}
	wantCanonical, err := estimateHuggingFaceCanonicalArtifactBytes(reservationRemoteFiles(configPayload, modelPayload))
	if err != nil {
		t.Fatalf("estimateHuggingFaceCanonicalArtifactBytes() error = %v", err)
	}
	wantSource, err := strictRemoteObjectFilesSize(reservationRemoteFiles(configPayload, modelPayload))
	if err != nil {
		t.Fatalf("strictRemoteObjectFilesSize() error = %v", err)
	}
	wantSize, err := addStorageBytes(wantSource, wantCanonical)
	if err != nil {
		t.Fatalf("addStorageBytes() error = %v", err)
	}
	if request.SizeBytes != wantSize {
		t.Fatalf("reservation size bytes = %d, want %d", request.SizeBytes, wantSize)
	}
}

func TestHuggingFaceStorageReservationEstimatesCanonicalArtifactGrowth(t *testing.T) {
	t.Parallel()

	files := reservationRemoteFiles([]byte("123"), []byte("4567"))
	sourceBytes, err := strictRemoteObjectFilesSize(files)
	if err != nil {
		t.Fatalf("strictRemoteObjectFilesSize() error = %v", err)
	}
	directBytes, err := huggingFaceStorageReservationBytes(files, false)
	if err != nil {
		t.Fatalf("huggingFaceStorageReservationBytes(direct) error = %v", err)
	}
	mirrorBytes, err := huggingFaceStorageReservationBytes(files, true)
	if err != nil {
		t.Fatalf("huggingFaceStorageReservationBytes(mirror) error = %v", err)
	}

	if directBytes <= sourceBytes {
		t.Fatalf("direct reservation bytes = %d, want greater than raw source bytes %d", directBytes, sourceBytes)
	}
	if mirrorBytes <= sourceBytes*2 {
		t.Fatalf("mirror reservation bytes = %d, want greater than two raw copies %d", mirrorBytes, sourceBytes*2)
	}
}

func TestFetchRemoteModelHuggingFaceStopsMirrorTransferWhenReservationFails(t *testing.T) {
	server := reservationTestServer(t, []byte(`{"architectures":["LlamaForCausalLM"]}`), []byte("tensor-payload"))
	defer server.Close()

	stubDefaultHuggingFaceInfo(t)
	withHuggingFaceBaseURL(t, server.URL)
	stubHuggingFaceProfileSummary(t, defaultHuggingFaceProfileSummary(), nil)

	mirrorStore := &fakeSourceMirrorStore{}
	mirrorClient := newFakeMirrorUploadStaging(t)
	reservations := &fakeRemoteStorageReservation{err: errors.New("no space")}
	_, err := fetchTestHuggingFaceRemoteWithReservation(t, newTestSourceMirrorOptions(mirrorClient, mirrorStore), reservations)
	if err == nil {
		t.Fatal("FetchRemoteModel() error = nil, want reservation failure")
	}
	if got := len(mirrorClient.objects); got != 0 {
		t.Fatalf("mirror transfer should not start after reservation failure, got %d objects", got)
	}
}

func reservationTestServer(t *testing.T, configPayload []byte, modelPayload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requireHuggingFaceAuth(t, request)
		switch request.URL.Path {
		case "/owner/model/resolve/deadbeef/config.json":
			writer.Header().Set("Content-Length", strconv.Itoa(len(configPayload)))
			_, _ = writer.Write(configPayload)
		case "/owner/model/resolve/deadbeef/model.safetensors":
			writer.Header().Set("Content-Length", strconv.Itoa(len(modelPayload)))
			_, _ = writer.Write(modelPayload)
		default:
			http.NotFound(writer, request)
		}
	}))
}

func reservationRemoteFiles(configPayload []byte, modelPayload []byte) []RemoteObjectFile {
	return []RemoteObjectFile{
		{TargetPath: "config.json", SizeBytes: int64(len(configPayload))},
		{TargetPath: "model.safetensors", SizeBytes: int64(len(modelPayload))},
	}
}
