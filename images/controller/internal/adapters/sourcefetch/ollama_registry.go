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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/domain/modelsource"
)

const (
	ollamaDefaultTag                  = "latest"
	ollamaModelLayerMediaType         = "application/vnd.ollama.image.model"
	ollamaLicenseLayerMediaType       = "application/vnd.ollama.image.license"
	ollamaParamsLayerMediaType        = "application/vnd.ollama.image.params"
	ollamaDockerManifestV2            = "application/vnd.docker.distribution.manifest.v2+json"
	ollamaOCIManifestV1               = "application/vnd.oci.image.manifest.v1+json"
	ollamaConfigMaxBytes        int64 = 1 << 20
	ollamaLicenseMaxBytes       int64 = 256 << 10
	ollamaParamsMaxBytes        int64 = 256 << 10
	ollamaGGUFProbeBytes        int64 = 4
)

var (
	ollamaRegistryBaseURL = "https://registry.ollama.ai"
	ollamaDigestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type ollamaReference struct {
	Name              string
	Tag               string
	Subject           string
	ExternalReference string
	RegistryPath      string
}

type ollamaManifest struct {
	SchemaVersion int                `json:"schemaVersion"`
	MediaType     string             `json:"mediaType"`
	Config        ollamaDescriptor   `json:"config"`
	Layers        []ollamaDescriptor `json:"layers"`
}

type ollamaDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

type ollamaConfig struct {
	ModelFormat   string   `json:"model_format"`
	ModelFamily   string   `json:"model_family"`
	ModelFamilies []string `json:"model_families"`
	ModelType     string   `json:"model_type"`
	FileType      string   `json:"file_type"`
	Renderer      string   `json:"renderer"`
	Parser        string   `json:"parser"`
}

type ollamaParams struct {
	NumCtx int64 `json:"num_ctx"`
}

type ollamaRegistryClient struct {
	baseURL    string
	httpClient *http.Client
}

func parseOllamaReference(rawURL string) (ollamaReference, error) {
	name, tag, err := modelsource.ParseOllamaLibraryURL(rawURL)
	if err != nil {
		return ollamaReference{}, err
	}
	if strings.TrimSpace(tag) == "" {
		tag = ollamaDefaultTag
	}
	subject := "library/" + strings.Trim(strings.TrimSpace(name), "/")
	return ollamaReference{
		Name:              strings.TrimSpace(name),
		Tag:               strings.TrimSpace(tag),
		Subject:           subject,
		ExternalReference: "ollama.com/" + subject + ":" + strings.TrimSpace(tag),
		RegistryPath:      "library/" + strings.TrimSpace(name),
	}, nil
}

func (c ollamaRegistryClient) fetchManifest(ctx context.Context, ref ollamaReference) (ollamaManifest, error) {
	rawURL := c.registryURL("/v2/" + pathEscape(ref.RegistryPath) + "/manifests/" + url.PathEscape(ref.Tag))
	response, err := doGET(ctx, c.httpClient, rawURL, map[string]string{
		"Accept": ollamaDockerManifestV2 + ", " + ollamaOCIManifestV1,
	})
	if err != nil {
		return ollamaManifest{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ollamaManifest{}, unexpectedStatusError(response, "ollama manifest request")
	}

	var manifest ollamaManifest
	if err := decodeJSONResponse(response, &manifest); err != nil {
		return ollamaManifest{}, err
	}
	if err := validateOllamaManifest(manifest); err != nil {
		return ollamaManifest{}, err
	}
	return manifest, nil
}

func (c ollamaRegistryClient) fetchBlobBytes(ctx context.Context, ref ollamaReference, descriptor ollamaDescriptor, maxBytes int64) ([]byte, error) {
	if err := validateOllamaDescriptor(descriptor, "ollama blob"); err != nil {
		return nil, err
	}
	if maxBytes > 0 && descriptor.Size > maxBytes {
		return nil, fmt.Errorf("ollama blob %s size %d exceeds limit %d", descriptor.MediaType, descriptor.Size, maxBytes)
	}

	response, err := doGET(ctx, c.httpClient, c.blobURL(ref, descriptor.Digest), nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, unexpectedStatusError(response, "ollama blob request")
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, descriptor.Size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) != descriptor.Size {
		return nil, fmt.Errorf("ollama blob %s size %d does not match descriptor size %d", descriptor.Digest, len(payload), descriptor.Size)
	}
	if err := verifySHA256Digest(descriptor.Digest, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (c ollamaRegistryClient) probeGGUF(ctx context.Context, ref ollamaReference, descriptor ollamaDescriptor) error {
	reader := ollamaObjectReader{httpClient: c.httpClient}
	opened, err := reader.OpenReadRange(ctx, c.blobURL(ref, descriptor.Digest), 0, ollamaGGUFProbeBytes)
	if err != nil {
		return err
	}
	defer opened.Body.Close()
	header := make([]byte, ollamaGGUFProbeBytes)
	if _, err := io.ReadFull(opened.Body, header); err != nil {
		return err
	}
	if string(header) != "GGUF" {
		return fmt.Errorf("ollama model layer %s is not a GGUF blob", descriptor.Digest)
	}
	return nil
}

func (c ollamaRegistryClient) blobURL(ref ollamaReference, digest string) string {
	return c.registryURL("/v2/" + pathEscape(ref.RegistryPath) + "/blobs/" + strings.TrimSpace(digest))
}

func (c ollamaRegistryClient) registryURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(c.baseURL), "/")
	if base == "" {
		base = ollamaRegistryBaseURL
	}
	return base + path
}

func validateOllamaManifest(manifest ollamaManifest) error {
	if manifest.SchemaVersion != 2 {
		return fmt.Errorf("ollama manifest schemaVersion %d is not supported", manifest.SchemaVersion)
	}
	switch strings.TrimSpace(manifest.MediaType) {
	case "", ollamaDockerManifestV2, ollamaOCIManifestV1:
	default:
		return fmt.Errorf("ollama manifest media type %q is not supported", manifest.MediaType)
	}
	if err := validateOllamaDescriptor(manifest.Config, "ollama config descriptor"); err != nil {
		return err
	}
	if _, err := selectOllamaModelLayer(manifest); err != nil {
		return err
	}
	return nil
}

func selectOllamaModelLayer(manifest ollamaManifest) (ollamaDescriptor, error) {
	var selected *ollamaDescriptor
	for index := range manifest.Layers {
		layer := manifest.Layers[index]
		if strings.TrimSpace(layer.MediaType) != ollamaModelLayerMediaType {
			continue
		}
		if selected != nil {
			return ollamaDescriptor{}, errors.New("ollama manifest contains multiple model layers")
		}
		selected = &layer
	}
	if selected == nil {
		return ollamaDescriptor{}, errors.New("ollama manifest does not contain a model layer")
	}
	if err := validateOllamaDescriptor(*selected, "ollama model layer"); err != nil {
		return ollamaDescriptor{}, err
	}
	return *selected, nil
}

func selectOllamaLayer(manifest ollamaManifest, mediaType string) (ollamaDescriptor, bool, error) {
	var selected *ollamaDescriptor
	for index := range manifest.Layers {
		layer := manifest.Layers[index]
		if strings.TrimSpace(layer.MediaType) != mediaType {
			continue
		}
		if selected != nil {
			return ollamaDescriptor{}, false, fmt.Errorf("ollama manifest contains multiple %s layers", mediaType)
		}
		selected = &layer
	}
	if selected == nil {
		return ollamaDescriptor{}, false, nil
	}
	if err := validateOllamaDescriptor(*selected, "ollama "+mediaType+" layer"); err != nil {
		return ollamaDescriptor{}, false, err
	}
	return *selected, true, nil
}

func validateOllamaDescriptor(descriptor ollamaDescriptor, subject string) error {
	if strings.TrimSpace(descriptor.MediaType) == "" {
		return fmt.Errorf("%s media type must not be empty", subject)
	}
	if !ollamaDigestPattern.MatchString(strings.TrimSpace(descriptor.Digest)) {
		return fmt.Errorf("%s digest %q is not a sha256 digest", subject, descriptor.Digest)
	}
	if descriptor.Size <= 0 {
		return fmt.Errorf("%s size must be greater than zero", subject)
	}
	return nil
}

func decodeOllamaConfig(payload []byte) (ollamaConfig, error) {
	var config ollamaConfig
	if err := json.Unmarshal(payload, &config); err != nil {
		return ollamaConfig{}, err
	}
	if strings.ToLower(strings.TrimSpace(config.ModelFormat)) != "gguf" {
		return ollamaConfig{}, fmt.Errorf("ollama model format %q is not supported", config.ModelFormat)
	}
	return config, nil
}

func decodeOllamaParams(payload []byte) (ollamaParams, error) {
	var params ollamaParams
	if len(payload) == 0 {
		return params, nil
	}
	if err := json.Unmarshal(payload, &params); err != nil {
		return ollamaParams{}, err
	}
	return params, nil
}

func verifySHA256Digest(expected string, payload []byte) error {
	clean := strings.TrimSpace(expected)
	if !ollamaDigestPattern.MatchString(clean) {
		return fmt.Errorf("digest %q is not a sha256 digest", expected)
	}
	sum := sha256.Sum256(payload)
	actual := "sha256:" + hex.EncodeToString(sum[:])
	if actual != clean {
		return fmt.Errorf("blob digest mismatch: got %s, want %s", actual, clean)
	}
	return nil
}

func parseOllamaParameterCount(raw string) int64 {
	value := strings.TrimSpace(strings.TrimSuffix(strings.ToUpper(raw), "B"))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(parsed * 1_000_000_000)
}

func pathEscape(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for index := range parts {
		parts[index] = url.PathEscape(parts[index])
	}
	return strings.Join(parts, "/")
}

type ollamaObjectReader struct {
	httpClient *http.Client
}

func (r ollamaObjectReader) OpenRead(ctx context.Context, sourcePath string) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, sourcePath, 0, -1)
}

func (r ollamaObjectReader) OpenReadRange(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	return r.openRead(ctx, sourcePath, offset, length)
}

func (r ollamaObjectReader) openRead(ctx context.Context, sourcePath string, offset, length int64) (RemoteOpenReadResult, error) {
	headers := map[string]string{}
	if rangeHeader, ok := httpByteRangeHeader(offset, length); ok {
		headers["Range"] = rangeHeader
	}
	response, err := doGET(ctx, r.httpClient, strings.TrimSpace(sourcePath), headers)
	if err != nil {
		return RemoteOpenReadResult{}, err
	}
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		defer response.Body.Close()
		return RemoteOpenReadResult{}, unexpectedStatusError(response, "ollama object-source GET request")
	}
	sizeBytes, err := responseBodyLength(response)
	if err != nil {
		_ = response.Body.Close()
		return RemoteOpenReadResult{}, err
	}
	return RemoteOpenReadResult{
		Body:      response.Body,
		SizeBytes: sizeBytes,
		ETag:      strings.TrimSpace(response.Header.Get("ETag")),
	}, nil
}
