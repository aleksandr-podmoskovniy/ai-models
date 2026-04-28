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
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultInitContainerName = deliverycontract.DefaultMaterializerInitContainerName
	DefaultCacheMountPath    = "/data/modelcache"
	DefaultManagedCacheName  = "ai-models-node-cache"
	NodeCacheCSIDriverName   = nodecache.CSIDriverName

	ResolvedDigestAnnotation         = deliverycontract.ResolvedDigestAnnotation
	ResolvedArtifactURIAnnotation    = deliverycontract.ResolvedArtifactURIAnnotation
	ResolvedArtifactFamilyAnnotation = deliverycontract.ResolvedArtifactFamilyAnnotation
	ResolvedDeliveryModeAnnotation   = deliverycontract.ResolvedDeliveryModeAnnotation
	ResolvedDeliveryReasonAnnotation = deliverycontract.ResolvedDeliveryReasonAnnotation
	ResolvedModelsAnnotation         = deliverycontract.ResolvedModelsAnnotation

	ModelPathEnv     = "AI_MODELS_MODEL_PATH"
	ModelDigestEnv   = "AI_MODELS_MODEL_DIGEST"
	ModelFamilyEnv   = "AI_MODELS_MODEL_FAMILY"
	ModelsDirEnv     = "AI_MODELS_MODELS_DIR"
	ModelsEnv        = "AI_MODELS_MODELS"
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

func ModelsDirPath(options Options) string {
	return nodecache.WorkloadModelsDirPath(strings.TrimSpace(options.CacheMountPath))
}

func NamedModelPath(options Options, alias string) string {
	return nodecache.WorkloadModelAliasPath(strings.TrimSpace(options.CacheMountPath), alias)
}

func NamedModelPathEnv(alias string) string {
	return namedModelEnv(alias, "PATH")
}

func NamedModelDigestEnv(alias string) string {
	return namedModelEnv(alias, "DIGEST")
}

func NamedModelFamilyEnv(alias string) string {
	return namedModelEnv(alias, "FAMILY")
}

func namedModelEnv(alias, suffix string) string {
	alias = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(alias), "-", "_"))
	return fmt.Sprintf("AI_MODELS_MODEL_%s_%s", alias, suffix)
}

func managedInitContainerName(baseName, alias string) string {
	baseName = strings.TrimSpace(baseName)
	alias = strings.TrimSpace(alias)
	name := baseName + "-" + alias
	if len(name) <= 63 {
		return name
	}
	sum := sha1.Sum([]byte(alias))
	shortHash := hex.EncodeToString(sum[:])[:10]
	prefixBudget := 63 - len(shortHash) - 1
	if prefixBudget > len(baseName) {
		prefixBudget = len(baseName)
	}
	if prefixBudget < 1 {
		prefixBudget = 1
	}
	return baseName[:prefixBudget] + "-" + shortHash
}

func managedModelVolumeName(baseName, alias string) string {
	return managedInitContainerName(baseName, alias)
}

type DeliveryMode string

const (
	DeliveryModeMaterializeBridge DeliveryMode = deliverycontract.DeliveryModeMaterializeBridge
	DeliveryModeSharedPVCBridge   DeliveryMode = deliverycontract.DeliveryModeSharedPVCBridge
	DeliveryModeSharedDirect      DeliveryMode = deliverycontract.DeliveryModeSharedDirect
)

type DeliveryReason string

const (
	DeliveryReasonWorkloadCacheVolume            DeliveryReason = deliverycontract.DeliveryReasonWorkloadCacheVolume
	DeliveryReasonManagedBridgeVolume            DeliveryReason = deliverycontract.DeliveryReasonManagedBridgeVolume
	DeliveryReasonStatefulSetClaimTemplate       DeliveryReason = deliverycontract.DeliveryReasonStatefulSetClaimTemplate
	DeliveryReasonWorkloadSharedPersistentVolume DeliveryReason = deliverycontract.DeliveryReasonWorkloadSharedPersistentVolume
	DeliveryReasonNodeSharedRuntimePlane         DeliveryReason = deliverycontract.DeliveryReasonNodeSharedRuntimePlane
)
