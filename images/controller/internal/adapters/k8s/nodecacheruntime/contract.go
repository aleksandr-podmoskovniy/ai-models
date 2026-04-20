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

package nodecacheruntime

const (
	ManagedLabelKey        = "ai.deckhouse.io/node-cache-runtime"
	ManagedLabelValue      = "managed"
	NodeNameAnnotationKey  = "ai.deckhouse.io/node-cache-runtime-node-name"
	RuntimeNodeNameEnv     = "AI_MODELS_NODE_NAME"
	DefaultContainerName   = "node-cache-runtime"
	registryCAPath         = "/etc/ai-models/registry-ca"
	registryCAFilePath     = "/etc/ai-models/registry-ca/ca.crt"
	registryCASecretVolume = "registry-ca"
	cacheRootVolumeName    = "cache-root"
)

type RuntimeSpec struct {
	Namespace           string
	NodeName            string
	RuntimeImage        string
	ImagePullSecretName string
	ServiceAccountName  string
	StorageClassName    string
	SharedVolumeSize    string
	MaxTotalSize        string
	MaxUnusedAge        string
	ScanInterval        string
	OCIInsecure         bool
	OCIAuthSecretName   string
	OCIRegistryCASecret string
}
