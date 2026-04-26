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

package modeldelivery

import (
	"errors"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultInitContainerName = "ai-models-materializer"
	DefaultCacheMountPath    = "/data/modelcache"
	DefaultManagedCacheName  = "ai-models-node-cache"
	NodeCacheCSIDriverName   = nodecache.CSIDriverName

	ResolvedDigestAnnotation         = "ai.deckhouse.io/resolved-digest"
	ResolvedArtifactURIAnnotation    = "ai.deckhouse.io/resolved-artifact-uri"
	ResolvedArtifactFamilyAnnotation = "ai.deckhouse.io/resolved-artifact-family"
	ResolvedDeliveryModeAnnotation   = "ai.deckhouse.io/resolved-delivery-mode"
	ResolvedDeliveryReasonAnnotation = "ai.deckhouse.io/resolved-delivery-reason"

	ModelPathEnv     = "AI_MODELS_MODEL_PATH"
	ModelDigestEnv   = "AI_MODELS_MODEL_DIGEST"
	ModelFamilyEnv   = "AI_MODELS_MODEL_FAMILY"
	LogFormatEnv     = "LOG_FORMAT"
	LogLevelEnv      = "LOG_LEVEL"
	defaultLogFormat = "json"
	defaultLogLevel  = "info"

	nodeCacheCSIAttributeArtifactURI    = nodecache.CSIAttributeArtifactURI
	nodeCacheCSIAttributeArtifactDigest = nodecache.CSIAttributeArtifactDigest
	nodeCacheCSIAttributeArtifactFamily = nodecache.CSIAttributeArtifactFamily
)

type Options struct {
	RuntimeImage    string
	ImagePullPolicy corev1.PullPolicy
	LogFormat       string
	LogLevel        string
	OCIInsecure     bool

	InitContainerName string
	CacheMountPath    string
}

type ManagedCacheOptions struct {
	Enabled      bool
	VolumeName   string
	NodeSelector map[string]string
}

func NormalizeOptions(options Options) Options {
	if options.ImagePullPolicy == "" {
		options.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if strings.TrimSpace(options.LogFormat) == "" {
		options.LogFormat = defaultLogFormat
	}
	if strings.TrimSpace(options.LogLevel) == "" {
		options.LogLevel = defaultLogLevel
	}
	if strings.TrimSpace(options.InitContainerName) == "" {
		options.InitContainerName = DefaultInitContainerName
	}
	if strings.TrimSpace(options.CacheMountPath) == "" {
		options.CacheMountPath = DefaultCacheMountPath
	}
	return options
}

func ValidateOptions(options Options) error {
	switch {
	case strings.TrimSpace(options.RuntimeImage) == "":
		return errors.New("runtime delivery image must not be empty")
	case strings.TrimSpace(options.InitContainerName) == "":
		return errors.New("runtime delivery init container name must not be empty")
	case strings.TrimSpace(options.CacheMountPath) == "":
		return errors.New("runtime delivery cache mount path must not be empty")
	case !strings.HasPrefix(strings.TrimSpace(options.CacheMountPath), "/"):
		return errors.New("runtime delivery cache mount path must be absolute")
	}
	return nil
}

func NormalizeManagedCacheOptions(options ManagedCacheOptions) ManagedCacheOptions {
	if strings.TrimSpace(options.VolumeName) == "" {
		options.VolumeName = DefaultManagedCacheName
	}
	options.NodeSelector = copyCleanStringMap(options.NodeSelector)
	return options
}

func ValidateManagedCacheOptions(options ManagedCacheOptions) error {
	if !options.Enabled {
		return nil
	}
	switch {
	case strings.TrimSpace(options.VolumeName) == "":
		return errors.New("runtime delivery managed cache volume name must not be empty")
	}
	return nil
}

func copyCleanStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		output[key] = strings.TrimSpace(value)
	}
	return output
}

func ModelPath(options Options) string {
	return nodecache.WorkloadModelPath(strings.TrimSpace(options.CacheMountPath))
}

type DeliveryMode string

const (
	DeliveryModeMaterializeBridge DeliveryMode = "MaterializeBridge"
	DeliveryModeSharedPVCBridge   DeliveryMode = "SharedPVCBridge"
	DeliveryModeSharedDirect      DeliveryMode = "SharedDirect"
)

type DeliveryReason string

const (
	DeliveryReasonWorkloadCacheVolume            DeliveryReason = "WorkloadCacheVolume"
	DeliveryReasonManagedBridgeVolume            DeliveryReason = "ManagedBridgeVolume"
	DeliveryReasonStatefulSetClaimTemplate       DeliveryReason = "StatefulSetClaimTemplate"
	DeliveryReasonWorkloadSharedPersistentVolume DeliveryReason = "WorkloadSharedPersistentVolumeClaim"
	DeliveryReasonNodeSharedRuntimePlane         DeliveryReason = "NodeSharedRuntimePlane"
)
