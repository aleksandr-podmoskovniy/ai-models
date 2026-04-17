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

package oci

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const ManifestAcceptHeader = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.artifact.manifest.v1+json, application/vnd.cncf.oras.artifact.manifest.v1+json"

const (
	ModelPackArtifactType       = "application/vnd.cncf.model.manifest.v1+json"
	ModelPackConfigMediaType    = "application/vnd.cncf.model.config.v1+json"
	ModelPackWeightLayerType    = "application/vnd.cncf.model.weight.v1.tar"
	ModelPackFilepathAnnotation = "org.cncf.model.filepath"
)

type InspectPayload map[string]any

func InspectRemote(ctx context.Context, reference string, auth modelpackports.RegistryAuth) (InspectPayload, error) {
	manifestURL, err := RegistryManifestURL(reference)
	if err != nil {
		return nil, err
	}

	client, err := RegistryHTTPClient(auth)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", ManifestAcceptHeader)
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query remote ModelPack manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("failed to query remote ModelPack manifest: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	digest := strings.TrimSpace(resp.Header.Get("Docker-Content-Digest"))
	if digest == "" {
		return nil, errors.New("registry manifest response is missing Docker-Content-Digest")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote ModelPack manifest body: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("registry manifest response body is empty")
	}

	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("failed to decode remote ModelPack manifest: %w", err)
	}

	configBlob, err := FetchConfigBlob(ctx, client, reference, auth, manifest)
	if err != nil {
		return nil, err
	}

	return InspectPayload{
		"digest":     digest,
		"manifest":   manifest,
		"configBlob": configBlob,
	}, nil
}

func RegistryManifestURL(reference string) (string, error) {
	parsed, err := parseOCIReference(reference)
	if err != nil {
		return "", err
	}

	return parsed.manifestURL(parsed.Reference), nil
}

func RegistryBlobURL(reference, digest string) (string, error) {
	parsed, err := parseOCIReference(reference)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(digest) == "" {
		return "", fmt.Errorf("invalid OCI reference %q", reference)
	}

	return parsed.blobURL(digest), nil
}

func RegistryHTTPClient(auth modelpackports.RegistryAuth) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: auth.Insecure} //nolint:gosec
	if strings.TrimSpace(auth.CAFile) != "" {
		caPEM, err := os.ReadFile(auth.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read OCI CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("failed to append OCI CA bundle")
		}
		tlsConfig.RootCAs = pool
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}

func FetchConfigBlob(
	ctx context.Context,
	client *http.Client,
	reference string,
	auth modelpackports.RegistryAuth,
	manifest map[string]any,
) (map[string]any, error) {
	configDescriptor, _ := manifest["config"].(map[string]any)
	if configDescriptor == nil {
		return nil, errors.New("registry manifest is missing config descriptor")
	}
	digest, _ := configDescriptor["digest"].(string)
	if strings.TrimSpace(digest) == "" {
		return nil, errors.New("registry manifest config descriptor is missing digest")
	}

	body, err := FetchBlob(ctx, client, reference, strings.TrimSpace(digest), auth)
	if err != nil {
		return nil, fmt.Errorf("failed to query remote ModelPack config blob: %w", err)
	}

	var configBlob map[string]any
	if err := json.Unmarshal(body, &configBlob); err != nil {
		return nil, fmt.Errorf("failed to decode remote ModelPack config blob: %w", err)
	}

	return configBlob, nil
}

func FetchBlob(ctx context.Context, client *http.Client, reference, digest string, auth modelpackports.RegistryAuth) ([]byte, error) {
	resp, err := GetBlobResponse(ctx, client, reference, digest, auth)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote blob: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("registry blob response body is empty")
	}

	return body, nil
}

func GetBlobResponse(ctx context.Context, client *http.Client, reference, digest string, auth modelpackports.RegistryAuth) (*http.Response, error) {
	blobURL, err := RegistryBlobURL(reference, digest)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query remote blob: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, fmt.Errorf("failed to query remote blob: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return resp, nil
}
