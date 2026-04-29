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

package workloaddelivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

var signedAnnotationKeys = []string{
	ResolvedDigestAnnotation,
	ResolvedArtifactURIAnnotation,
	ResolvedArtifactFamilyAnnotation,
	ResolvedDeliveryModeAnnotation,
	ResolvedDeliveryReasonAnnotation,
	ResolvedModelsAnnotation,
}

func SignResolvedDelivery(namespace string, annotations map[string]string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(strings.TrimSpace(namespace)))
	for _, annotationKey := range signedAnnotationKeys {
		mac.Write([]byte{0})
		mac.Write([]byte(annotationKey))
		mac.Write([]byte{0})
		mac.Write([]byte(strings.TrimSpace(annotations[annotationKey])))
	}
	return "hmac-sha256:" + hex.EncodeToString(mac.Sum(nil))
}

func VerifyResolvedDeliverySignature(namespace string, annotations map[string]string, key string) bool {
	expected := SignResolvedDelivery(namespace, annotations, key)
	if expected == "" {
		return false
	}
	actual := strings.TrimSpace(annotations[ResolvedSignatureAnnotation])
	if actual == "" {
		return false
	}
	return hmac.Equal([]byte(actual), []byte(expected))
}
