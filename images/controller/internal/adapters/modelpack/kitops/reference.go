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
	"os"
	"path/filepath"
	"strings"

	modelpackports "github.com/deckhouse/ai-models/controller/internal/ports/modelpack"
)

func connectionFlags(auth modelpackports.RegistryAuth) []string {
	if auth.Insecure {
		return []string{"--tls-verify=false"}
	}
	return nil
}

func runtimeEnvironment(configDir string, auth modelpackports.RegistryAuth) []string {
	environment := os.Environ()
	environment = append(environment,
		"HOME="+configDir,
		"DOCKER_CONFIG="+filepath.Join(configDir, "docker"),
	)
	if strings.TrimSpace(auth.CAFile) != "" {
		environment = append(environment, "SSL_CERT_FILE="+auth.CAFile)
	}
	return environment
}

func registryFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(reference)
	withoutDigest := strings.SplitN(cleanReference, "@", 2)[0]
	registry, repository, found := strings.Cut(withoutDigest, "/")
	if !found || strings.TrimSpace(registry) == "" || strings.TrimSpace(repository) == "" {
		return ""
	}
	return registry
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

func packageNameFromOCIReference(reference string) string {
	cleanReference := strings.TrimSpace(strings.SplitN(reference, "@", 2)[0])
	if index := strings.LastIndex(cleanReference, "/"); index >= 0 {
		cleanReference = cleanReference[index+1:]
	}
	if index := strings.LastIndex(cleanReference, ":"); index >= 0 {
		cleanReference = cleanReference[:index]
	}
	if cleanReference == "" {
		return "model"
	}
	return cleanReference
}
