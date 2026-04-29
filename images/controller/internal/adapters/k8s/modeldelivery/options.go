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
)

const (
	LegacyMaterializerInitContainerName = "ai-models-materializer"
	DefaultCacheMountPath               = "/data/modelcache"
	DefaultManagedCacheName             = "ai-models-node-cache"
	NodeCacheCSIDriverName              = nodecache.CSIDriverName

	ResolvedDigestAnnotation         = deliverycontract.ResolvedDigestAnnotation
	ResolvedArtifactURIAnnotation    = deliverycontract.ResolvedArtifactURIAnnotation
	ResolvedArtifactFamilyAnnotation = deliverycontract.ResolvedArtifactFamilyAnnotation
	ResolvedDeliveryModeAnnotation   = deliverycontract.ResolvedDeliveryModeAnnotation
	ResolvedDeliveryReasonAnnotation = deliverycontract.ResolvedDeliveryReasonAnnotation
	ResolvedModelsAnnotation         = deliverycontract.ResolvedModelsAnnotation
	ResolvedSignatureAnnotation      = deliverycontract.ResolvedSignatureAnnotation

	ModelPathEnv   = "AI_MODELS_MODEL_PATH"
	ModelDigestEnv = "AI_MODELS_MODEL_DIGEST"
	ModelFamilyEnv = "AI_MODELS_MODEL_FAMILY"
	ModelsDirEnv   = "AI_MODELS_MODELS_DIR"
	ModelsEnv      = "AI_MODELS_MODELS"

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

func managedResourceName(baseName, alias string) string {
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
	return managedResourceName(baseName, alias)
}

type DeliveryMode string

const (
	DeliveryModeSharedDirect DeliveryMode = deliverycontract.DeliveryModeSharedDirect
)

type DeliveryReason string

const (
	DeliveryReasonNodeSharedRuntimePlane DeliveryReason = deliverycontract.DeliveryReasonNodeSharedRuntimePlane
)

type DeliveryGateReason string

const (
	DeliveryGateReasonInsufficientNodeCacheCapacity DeliveryGateReason = "InsufficientNodeCacheCapacity"
	DeliveryGateReasonNodeCacheDeliveryDisabled     DeliveryGateReason = "NodeCacheDeliveryDisabled"
)
