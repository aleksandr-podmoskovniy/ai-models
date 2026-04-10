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

package resourcenames

import (
	"errors"
	"strings"

	"k8s.io/apimachinery/pkg/types"
)

const (
	sourceWorkerPodPrefix        = "ai-model-publish-"
	sourceWorkerAuthSecretPrefix = "ai-model-publish-auth-"
	ociRegistryAuthSecretPrefix  = "ai-model-oci-auth-"
	ociRegistryCASecretPrefix    = "ai-model-oci-ca-"
	uploadSessionPodPrefix       = "ai-model-upload-"
	uploadSessionServicePrefix   = "ai-model-upload-"
	uploadSessionIngressPrefix   = "ai-model-upload-"
	uploadSessionSecretPrefix    = "ai-model-upload-auth-"
	cleanupJobPrefix             = "ai-model-cleanup-"
	uploadStagingPrefix          = "uploaded-model-staging"

	AppNameLabelKey        = "app.kubernetes.io/name"
	OwnerKindLabelKey      = "ai-models.deckhouse.io/owner-kind"
	OwnerNameLabelKey      = "ai-models.deckhouse.io/owner-name"
	OwnerUIDLabelKey       = "ai-models.deckhouse.io/owner-uid"
	OwnerNamespaceLabelKey = "ai-models.deckhouse.io/owner-namespace"

	OwnerKindAnnotationKey      = "ai-models.deckhouse.io/owner-kind-full"
	OwnerNameAnnotationKey      = "ai-models.deckhouse.io/owner-name-full"
	OwnerNamespaceAnnotationKey = "ai-models.deckhouse.io/owner-namespace-full"
)

func PrefixedName(prefix string, uid types.UID) (string, error) {
	suffix, err := OwnerSuffix(uid)
	if err != nil {
		return "", err
	}
	return prefix + suffix, nil
}

func OwnerSuffix(uid types.UID) (string, error) {
	value := strings.TrimSpace(string(uid))
	if value == "" {
		return "", errors.New("owner UID must not be empty")
	}

	replacer := strings.NewReplacer("_", "-", ".", "-", ":", "-")
	value = replacer.Replace(strings.ToLower(value))
	if len(value) > 40 {
		value = value[:40]
	}
	value = strings.Trim(value, "-")
	if value == "" {
		return "", errors.New("owner UID normalized to an empty suffix")
	}
	return value, nil
}

func TruncateLabelValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 63 {
		return value
	}
	return strings.TrimRight(value[:63], "-_.")
}

func BoolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func SourceWorkerPodName(uid types.UID) (string, error) {
	return PrefixedName(sourceWorkerPodPrefix, uid)
}

func SourceWorkerAuthSecretName(uid types.UID) (string, error) {
	return PrefixedName(sourceWorkerAuthSecretPrefix, uid)
}

func OCIRegistryAuthSecretName(uid types.UID) (string, error) {
	return PrefixedName(ociRegistryAuthSecretPrefix, uid)
}

func OCIRegistryCASecretName(uid types.UID) (string, error) {
	return PrefixedName(ociRegistryCASecretPrefix, uid)
}

func UploadSessionPodName(uid types.UID) (string, error) {
	return PrefixedName(uploadSessionPodPrefix, uid)
}

func UploadSessionServiceName(uid types.UID) (string, error) {
	return PrefixedName(uploadSessionServicePrefix, uid)
}

func UploadSessionIngressName(uid types.UID) (string, error) {
	return PrefixedName(uploadSessionIngressPrefix, uid)
}

func UploadSessionSecretName(uid types.UID) (string, error) {
	return PrefixedName(uploadSessionSecretPrefix, uid)
}

func CleanupJobName(uid types.UID) (string, error) {
	return PrefixedName(cleanupJobPrefix, uid)
}

func UploadStagingObjectPrefix(uid types.UID) (string, error) {
	suffix, err := OwnerSuffix(uid)
	if err != nil {
		return "", err
	}
	return uploadStagingPrefix + "/" + suffix, nil
}

func OwnerLabels(appName, kind, name string, uid types.UID, namespace string) map[string]string {
	labels := map[string]string{
		AppNameLabelKey:   appName,
		OwnerKindLabelKey: TruncateLabelValue(kind),
		OwnerNameLabelKey: TruncateLabelValue(name),
		OwnerUIDLabelKey:  TruncateLabelValue(string(uid)),
	}
	if strings.TrimSpace(namespace) != "" {
		labels[OwnerNamespaceLabelKey] = TruncateLabelValue(namespace)
	}
	return labels
}

func OwnerAnnotations(kind, name, namespace string) map[string]string {
	annotations := map[string]string{
		OwnerKindAnnotationKey: strings.TrimSpace(kind),
		OwnerNameAnnotationKey: strings.TrimSpace(name),
	}
	if strings.TrimSpace(namespace) != "" {
		annotations[OwnerNamespaceAnnotationKey] = strings.TrimSpace(namespace)
	}
	return annotations
}

func OwnerUIDFromLabels(labels map[string]string) (types.UID, bool) {
	if len(labels) == 0 {
		return "", false
	}

	value := strings.TrimSpace(labels[OwnerUIDLabelKey])
	if value == "" {
		return "", false
	}

	return types.UID(value), true
}
