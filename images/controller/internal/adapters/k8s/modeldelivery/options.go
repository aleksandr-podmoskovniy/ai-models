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
	"strings"

	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	deliverycontract "github.com/deckhouse/ai-models/controller/internal/workloaddelivery"
)

const (
	LegacyMaterializerInitContainerName = "ai-models-materializer"
	DefaultCacheMountPath               = "/data/modelcache"
	DefaultManagedCacheName             = "ai-models-node-cache"
	DefaultSharedPVCVolumeName          = "ai-models-shared-pvc"
	NodeCacheCSIDriverName              = nodecache.CSIDriverName

	ResolvedDigestAnnotation         = deliverycontract.ResolvedDigestAnnotation
	ResolvedArtifactURIAnnotation    = deliverycontract.ResolvedArtifactURIAnnotation
	ResolvedArtifactFamilyAnnotation = deliverycontract.ResolvedArtifactFamilyAnnotation
	ResolvedDeliveryModeAnnotation   = deliverycontract.ResolvedDeliveryModeAnnotation
	ResolvedDeliveryReasonAnnotation = deliverycontract.ResolvedDeliveryReasonAnnotation
	ResolvedModelsAnnotation         = deliverycontract.ResolvedModelsAnnotation
	ResolvedSignatureAnnotation      = deliverycontract.ResolvedSignatureAnnotation

	legacyModelPathEnv   = "AI_MODELS_MODEL_PATH"
	legacyModelDigestEnv = "AI_MODELS_MODEL_DIGEST"
	legacyModelFamilyEnv = "AI_MODELS_MODEL_FAMILY"
	ModelsDirEnv         = "AI_MODELS_MODELS_DIR"
	ModelsEnv            = "AI_MODELS_MODELS"

	nodeCacheCSIAttributeArtifactURI    = nodecache.CSIAttributeArtifactURI
	nodeCacheCSIAttributeArtifactDigest = nodecache.CSIAttributeArtifactDigest
	nodeCacheCSIAttributeArtifactFamily = nodecache.CSIAttributeArtifactFamily
)

type Options struct {
	LegacyInitContainerName string
	CacheMountPath          string
}

type ManagedCacheOptions struct {
	Enabled          bool
	VolumeName       string
	CapacityBytes    int64
	RuntimeNamespace string
}

type SharedPVCOptions struct {
	StorageClassName string
	VolumeName       string
}

const DeliveryAuthKeyEnv = deliverycontract.DeliveryAuthKeyEnv

func NormalizeOptions(options Options) Options {
	if strings.TrimSpace(options.LegacyInitContainerName) == "" {
		options.LegacyInitContainerName = LegacyMaterializerInitContainerName
	}
	if strings.TrimSpace(options.CacheMountPath) == "" {
		options.CacheMountPath = DefaultCacheMountPath
	}
	return options
}

func ValidateOptions(options Options) error {
	switch {
	case strings.TrimSpace(options.LegacyInitContainerName) == "":
		return errors.New("runtime delivery legacy init container name must not be empty")
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
	options.RuntimeNamespace = strings.TrimSpace(options.RuntimeNamespace)
	return options
}

func NormalizeSharedPVCOptions(options SharedPVCOptions) SharedPVCOptions {
	options.StorageClassName = strings.TrimSpace(options.StorageClassName)
	if strings.TrimSpace(options.VolumeName) == "" {
		options.VolumeName = DefaultSharedPVCVolumeName
	}
	return options
}

func ValidateManagedCacheOptions(options ManagedCacheOptions) error {
	if !options.Enabled {
		return nil
	}
	switch {
	case strings.TrimSpace(options.VolumeName) == "":
		return errors.New("runtime delivery managed cache volume name must not be empty")
	case options.CapacityBytes < 0:
		return errors.New("runtime delivery managed cache capacity bytes must not be negative")
	}
	return nil
}

func ValidateSharedPVCOptions(options SharedPVCOptions) error {
	options = NormalizeSharedPVCOptions(options)
	if strings.TrimSpace(options.VolumeName) == "" {
		return errors.New("runtime delivery shared PVC volume name must not be empty")
	}
	return nil
}

func ModelsDirPath(options Options) string {
	return nodecache.WorkloadModelsDirPath(strings.TrimSpace(options.CacheMountPath))
}

func NamedModelPath(options Options, name string) string {
	return nodecache.WorkloadNamedModelPath(strings.TrimSpace(options.CacheMountPath), name)
}

func managedResourceName(baseName, name string) string {
	baseName = strings.TrimSpace(baseName)
	name = strings.TrimSpace(name)
	suffix := dnsLabelSuffix(name)
	candidate := baseName + "-" + suffix
	if suffix == strings.ToLower(name) && len(candidate) <= 63 {
		return candidate
	}
	sum := sha1.Sum([]byte(name))
	shortHash := hex.EncodeToString(sum[:])[:10]
	suffixBudget := 63 - len(baseName) - len(shortHash) - 2
	if suffixBudget > 0 {
		return baseName + "-" + dnsLabelPrefix(suffix, suffixBudget) + "-" + shortHash
	}
	return dnsLabelPrefix(baseName, 63-len(shortHash)-1) + "-" + shortHash
}

func managedModelVolumeName(baseName, name string) string {
	return managedResourceName(baseName, name)
}

func dnsLabelSuffix(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(".", "-", "_", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "model"
	}
	return value
}

func dnsLabelPrefix(value string, limit int) string {
	value = dnsLabelSuffix(value)
	if limit < 1 {
		return "m"
	}
	if len(value) > limit {
		value = value[:limit]
	}
	value = strings.Trim(value, "-")
	if value == "" {
		return "m"
	}
	return value
}

type DeliveryMode string

const (
	DeliveryModeSharedDirect DeliveryMode = deliverycontract.DeliveryModeSharedDirect
	DeliveryModeSharedPVC    DeliveryMode = deliverycontract.DeliveryModeSharedPVC
)

type DeliveryReason string

const (
	DeliveryReasonNodeSharedRuntimePlane DeliveryReason = deliverycontract.DeliveryReasonNodeSharedRuntimePlane
	DeliveryReasonRWXSharedVolume        DeliveryReason = deliverycontract.DeliveryReasonRWXSharedVolume
)

type DeliveryGateReason string

const (
	DeliveryGateReasonInsufficientNodeCacheCapacity DeliveryGateReason = "InsufficientNodeCacheCapacity"
	DeliveryGateReasonNodeCacheDeliveryDisabled     DeliveryGateReason = "NodeCacheDeliveryDisabled"
	DeliveryGateReasonSharedPVCStorageClassMissing  DeliveryGateReason = "SharedPVCStorageClassMissing"
	DeliveryGateReasonSharedPVCClaimPending         DeliveryGateReason = "SharedPVCClaimPending"
	DeliveryGateReasonSharedPVCMaterializerPending  DeliveryGateReason = "SharedPVCMaterializerPending"
)
