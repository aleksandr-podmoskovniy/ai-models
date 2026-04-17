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
	"fmt"
	"net/url"
	"strings"
)

type registryReference struct {
	Registry   string
	Repository string
	Reference  string
}

func parseOCIReference(reference string) (registryReference, error) {
	cleanReference := strings.TrimSpace(reference)
	registry, repositoryAndTag, found := strings.Cut(cleanReference, "/")
	if !found || strings.TrimSpace(registry) == "" || strings.TrimSpace(repositoryAndTag) == "" {
		return registryReference{}, fmt.Errorf("invalid OCI reference %q", reference)
	}

	repository := repositoryAndTag
	ref := "latest"
	if repositoryAndDigest, digest, hasDigest := strings.Cut(repositoryAndTag, "@"); hasDigest {
		repository = repositoryAndDigest
		ref = digest
	} else if index := strings.LastIndex(repositoryAndTag, ":"); index >= 0 {
		repository = repositoryAndTag[:index]
		ref = repositoryAndTag[index+1:]
	}
	if strings.TrimSpace(repository) == "" || strings.TrimSpace(ref) == "" {
		return registryReference{}, fmt.Errorf("invalid OCI reference %q", reference)
	}

	return registryReference{
		Registry:   registry,
		Repository: repository,
		Reference:  ref,
	}, nil
}

func (r registryReference) manifestURL(reference string) string {
	return (&url.URL{
		Scheme: "https",
		Host:   r.Registry,
		Path:   "/v2/" + r.Repository + "/manifests/" + url.PathEscape(strings.TrimSpace(reference)),
	}).String()
}

func (r registryReference) blobURL(digest string) string {
	return (&url.URL{
		Scheme: "https",
		Host:   r.Registry,
		Path:   "/v2/" + r.Repository + "/blobs/" + strings.TrimSpace(digest),
	}).String()
}

func (r registryReference) uploadURL() string {
	return (&url.URL{
		Scheme: "https",
		Host:   r.Registry,
		Path:   "/v2/" + r.Repository + "/blobs/uploads/",
	}).String()
}

func immutableOCIReference(reference, digest string) string {
	cleanReference := strings.TrimSpace(reference)
	cleanDigest := strings.TrimSpace(digest)
	if cleanReference == "" || cleanDigest == "" {
		return ""
	}

	withoutDigest := strings.SplitN(cleanReference, "@", 2)[0]
	repositoryPart := withoutDigest[strings.LastIndex(withoutDigest, "/")+1:]
	if strings.Contains(repositoryPart, ":") {
		withoutDigest = withoutDigest[:strings.LastIndex(withoutDigest, ":")]
	}

	return withoutDigest + "@" + cleanDigest
}

func resolveUploadLocation(baseURL, location string) (string, error) {
	uploadLocation := strings.TrimSpace(location)
	if uploadLocation == "" {
		return "", fmt.Errorf("registry upload response is missing Location header")
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsedLocation, err := url.Parse(uploadLocation)
	if err != nil {
		return "", err
	}

	return parsedBase.ResolveReference(parsedLocation).String(), nil
}
