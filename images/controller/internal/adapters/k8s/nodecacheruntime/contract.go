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

import "github.com/deckhouse/ai-models/controller/internal/support/resourcenames"

const (
	ManagedLabelKey        = "ai.deckhouse.io/node-cache-runtime"
	ManagedLabelValue      = "managed"
	NodeNameAnnotationKey  = "ai.deckhouse.io/node-cache-runtime-node-name"
	RuntimeNodeNameEnv     = "AI_MODELS_NODE_NAME"
	DefaultContainerName   = "node-cache-runtime"
	RegistrarContainerName = "node-driver-registrar"
	registryCAPath         = "/etc/ai-models/registry-ca"
	registryCAFilePath     = "/etc/ai-models/registry-ca/ca.crt"
	registryCASecretVolume = "registry-ca"
	cacheRootVolumeName    = "cache-root"
	csiPluginVolumeName    = "plugin-dir"
	csiRegistryVolumeName  = "registration-dir"
	kubeletVolumeName      = "kubelet-dir"
	deviceVolumeName       = "device-dir"
	csiPluginMountPath     = "/csi"
	csiRegistryMountPath   = "/registration"
	kubeletHostPath        = "/var/lib/kubelet"
	deviceHostPath         = "/dev"
)

type RuntimeSpec struct {
	Namespace           string
	NodeName            string
	NodeHostname        string
	RuntimeImage        string
	CSIRegistrarImage   string
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

func PodName(nodeName string) (string, error) {
	return resourcenames.NodeCacheRuntimePodName(nodeName)
}

func PVCName(nodeName string) (string, error) {
	return resourcenames.NodeCacheRuntimePVCName(nodeName)
}
