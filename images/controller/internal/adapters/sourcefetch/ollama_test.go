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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	modelsv1alpha1 "github.com/deckhouse/ai-models/api/core/v1alpha1"
)

func TestFetchRemoteModelOllamaPlansDirectGGUFObjectSource(t *testing.T) {
	fixture := newOllamaRegistryFixture(t)
	withOllamaRegistryBaseURL(t, fixture.server.URL)

	reservation := &fakeRemoteStorageReservation{}
	result, err := FetchRemoteModel(context.Background(), RemoteOptions{
		URL:                      "https://ollama.com/library/qwen3.6",
		StorageReservation:       reservation,
		SkipLocalMaterialization: true,
	})
	if err != nil {
		t.Fatalf("FetchRemoteModel() error = %v", err)
	}

	if got, want := result.SourceType, modelsv1alpha1.ModelSourceTypeOllama; got != want {
		t.Fatalf("unexpected source type %q", got)
	}
	if got, want := result.InputFormat, modelsv1alpha1.ModelInputFormatGGUF; got != want {
		t.Fatalf("unexpected input format %q", got)
	}
	if result.ObjectSource == nil {
		t.Fatal("expected direct object source")
	}
	if got, want := len(result.ObjectSource.Files), 1; got != want {
		t.Fatalf("unexpected object source file count %d", got)
	}
	file := result.ObjectSource.Files[0]
	if got, want := file.TargetPath, "qwen3.6-latest-q4_k_m.gguf"; got != want {
		t.Fatalf("unexpected target path %q", got)
	}
	if got, want := file.SizeBytes, int64(len(fixture.modelPayload)); got != want {
		t.Fatalf("unexpected model size %d", got)
	}
	if result.ProfileSummary == nil {
		t.Fatal("expected profile summary")
	}
	if got, want := result.ProfileSummary.Family, "qwen35moe"; got != want {
		t.Fatalf("unexpected family %q", got)
	}
	if got := result.ProfileSummary.Architecture; got != "" {
		t.Fatalf("ollama renderer architecture leaked into profile summary: %q", got)
	}
	if got, want := result.ProfileSummary.ParameterCount, int64(36_000_000_000); got != want {
		t.Fatalf("unexpected parameter count %d", got)
	}
	if got, want := result.ProfileSummary.Quantization, "Q4_K_M"; got != want {
		t.Fatalf("unexpected quantization %q", got)
	}
	if got, want := result.ProfileSummary.ContextWindowTokens, int64(8192); got != want {
		t.Fatalf("unexpected context window %d", got)
	}
	if got, want := reservation.requests[0].SourceType, modelsv1alpha1.ModelSourceTypeOllama; got != want {
		t.Fatalf("unexpected reserved source type %q", got)
	}
}

func TestFetchRemoteModelOllamaRejectsManifestWithoutModelLayer(t *testing.T) {
	fixture := newOllamaRegistryFixture(t)
	fixture.manifest.Layers = fixture.manifest.Layers[1:]
	withOllamaRegistryBaseURL(t, fixture.server.URL)

	_, err := FetchRemoteModel(context.Background(), RemoteOptions{
		URL:                      "https://ollama.com/library/qwen3.6",
		SkipLocalMaterialization: true,
	})
	if err == nil || !strings.Contains(err.Error(), "does not contain a model layer") {
		t.Fatalf("FetchRemoteModel() error = %v, want missing model layer", err)
	}
}

func TestFetchRemoteModelOllamaRejectsMultipleModelLayers(t *testing.T) {
	fixture := newOllamaRegistryFixture(t)
	fixture.manifest.Layers = append(fixture.manifest.Layers, fixture.manifest.Layers[0])
	withOllamaRegistryBaseURL(t, fixture.server.URL)

	_, err := FetchRemoteModel(context.Background(), RemoteOptions{
		URL:                      "https://ollama.com/library/qwen3.6",
		SkipLocalMaterialization: true,
	})
	if err == nil || !strings.Contains(err.Error(), "multiple model layers") {
		t.Fatalf("FetchRemoteModel() error = %v, want multiple model layers", err)
	}
}

func TestFetchRemoteModelOllamaRejectsInvalidDigest(t *testing.T) {
	fixture := newOllamaRegistryFixture(t)
	fixture.manifest.Layers[0].Digest = "sha256:not-a-digest"
	withOllamaRegistryBaseURL(t, fixture.server.URL)

	_, err := FetchRemoteModel(context.Background(), RemoteOptions{
		URL:                      "https://ollama.com/library/qwen3.6",
		SkipLocalMaterialization: true,
	})
	if err == nil || !strings.Contains(err.Error(), "is not a sha256 digest") {
		t.Fatalf("FetchRemoteModel() error = %v, want invalid digest", err)
	}
}

func newOllamaRegistryFixture(t *testing.T) *ollamaRegistryFixture {
	t.Helper()

	fixture := &ollamaRegistryFixture{
		configPayload:  []byte(`{"model_format":"gguf","model_family":"qwen35moe","model_type":"36.0B","file_type":"Q4_K_M","renderer":"qwen3.5"}`),
		paramsPayload:  []byte(`{"num_ctx":8192}`),
		licensePayload: []byte("Apache-2.0"),
		modelPayload:   []byte("GGUF tiny model payload"),
		blobs:          map[string][]byte{},
	}
	config := fixture.descriptor(ollamaDockerManifestV2, fixture.configPayload)
	model := fixture.descriptor(ollamaModelLayerMediaType, fixture.modelPayload)
	license := fixture.descriptor(ollamaLicenseLayerMediaType, fixture.licensePayload)
	params := fixture.descriptor(ollamaParamsLayerMediaType, fixture.paramsPayload)
	fixture.manifest = ollamaManifest{
		SchemaVersion: 2,
		MediaType:     ollamaDockerManifestV2,
		Config:        config,
		Layers:        []ollamaDescriptor{model, license, params},
	}
	fixture.server = httptest.NewServer(http.HandlerFunc(fixture.serve))
	t.Cleanup(fixture.server.Close)
	return fixture
}

type ollamaRegistryFixture struct {
	server         *httptest.Server
	manifest       ollamaManifest
	blobs          map[string][]byte
	configPayload  []byte
	paramsPayload  []byte
	licensePayload []byte
	modelPayload   []byte
}

func (f *ollamaRegistryFixture) descriptor(mediaType string, payload []byte) ollamaDescriptor {
	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	f.blobs[digest] = payload
	return ollamaDescriptor{
		MediaType: mediaType,
		Digest:    digest,
		Size:      int64(len(payload)),
	}
}

func (f *ollamaRegistryFixture) serve(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/v2/library/qwen3.6/manifests/latest":
		_ = json.NewEncoder(writer).Encode(f.manifest)
	case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/v2/library/qwen3.6/blobs/"):
		digest := strings.TrimPrefix(request.URL.Path, "/v2/library/qwen3.6/blobs/")
		payload, found := f.blobs[digest]
		if !found {
			http.NotFound(writer, request)
			return
		}
		writeBlobResponse(writer, request, payload)
	default:
		http.NotFound(writer, request)
	}
}

func writeBlobResponse(writer http.ResponseWriter, request *http.Request, payload []byte) {
	if rawRange := request.Header.Get("Range"); rawRange != "" {
		start, end := parseTestRange(rawRange, int64(len(payload)))
		writer.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(len(payload)))
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write(payload[start : end+1])
		return
	}
	writer.Header().Set("Content-Length", strconv.Itoa(len(payload)))
	_, _ = writer.Write(payload)
}

func parseTestRange(raw string, size int64) (int64, int64) {
	spec := strings.TrimPrefix(strings.TrimSpace(raw), "bytes=")
	startRaw, endRaw, _ := strings.Cut(spec, "-")
	start, _ := strconv.ParseInt(startRaw, 10, 64)
	if strings.TrimSpace(endRaw) == "" {
		return start, size - 1
	}
	end, _ := strconv.ParseInt(endRaw, 10, 64)
	return start, end
}

func withOllamaRegistryBaseURL(t *testing.T, baseURL string) {
	t.Helper()
	previous := ollamaRegistryBaseURL
	ollamaRegistryBaseURL = baseURL
	t.Cleanup(func() { ollamaRegistryBaseURL = previous })
}
