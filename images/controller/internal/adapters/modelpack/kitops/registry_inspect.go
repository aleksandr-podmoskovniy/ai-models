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

package kitops

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

const manifestAcceptHeader = "application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.artifact.manifest.v1+json, application/vnd.cncf.oras.artifact.manifest.v1+json"

const (
	modelPackArtifactType       = "application/vnd.cncf.model.manifest.v1+json"
	modelPackConfigMediaType    = "application/vnd.cncf.model.config.v1+json"
	modelPackWeightLayerType    = "application/vnd.cncf.model.weight.v1.tar"
	modelPackFilepathAnnotation = "org.cncf.model.filepath"
)

func inspectRemoteViaRegistry(ctx context.Context, reference string, auth modelpackports.RegistryAuth) (map[string]any, error) {
	manifestURL, err := registryManifestURL(reference)
	if err != nil {
		return nil, err
	}

	client, err := registryHTTPClient(auth)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestAcceptHeader)
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

	configBlob, err := fetchConfigBlob(ctx, client, reference, auth, manifest)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"digest":     digest,
		"manifest":   manifest,
		"configBlob": configBlob,
	}, nil
}

func registryManifestURL(reference string) (string, error) {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repositoryAndTag, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" || strings.TrimSpace(repositoryAndTag) == "" {
		return "", fmt.Errorf("invalid OCI reference %q", reference)
	}

	repository := repositoryAndTag
	ref := "latest"
	if index := strings.LastIndex(repositoryAndTag, ":"); index >= 0 {
		repository = repositoryAndTag[:index]
		ref = repositoryAndTag[index+1:]
	}
	if strings.TrimSpace(repository) == "" || strings.TrimSpace(ref) == "" {
		return "", fmt.Errorf("invalid OCI reference %q", reference)
	}

	return (&url.URL{
		Scheme: "https",
		Host:   registry,
		Path:   "/v2/" + repository + "/manifests/" + url.PathEscape(ref),
	}).String(), nil
}

func registryBlobURL(reference, digest string) (string, error) {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	registry, repositoryAndTag, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" || strings.TrimSpace(repositoryAndTag) == "" {
		return "", fmt.Errorf("invalid OCI reference %q", reference)
	}

	repository := repositoryAndTag
	if index := strings.LastIndex(repositoryAndTag, ":"); index >= 0 {
		repository = repositoryAndTag[:index]
	}
	if strings.TrimSpace(repository) == "" || strings.TrimSpace(digest) == "" {
		return "", fmt.Errorf("invalid OCI reference %q", reference)
	}

	return (&url.URL{
		Scheme: "https",
		Host:   registry,
		Path:   "/v2/" + repository + "/blobs/" + digest,
	}).String(), nil
}

func registryHTTPClient(auth modelpackports.RegistryAuth) (*http.Client, error) {
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

func fetchConfigBlob(
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

	blobURL, err := registryBlobURL(reference, strings.TrimSpace(digest))
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
		return nil, fmt.Errorf("failed to query remote ModelPack config blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("failed to query remote ModelPack config blob: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote ModelPack config blob: %w", err)
	}
	if len(body) == 0 {
		return nil, errors.New("registry config blob response body is empty")
	}

	var configBlob map[string]any
	if err := json.Unmarshal(body, &configBlob); err != nil {
		return nil, fmt.Errorf("failed to decode remote ModelPack config blob: %w", err)
	}

	return configBlob, nil
}
