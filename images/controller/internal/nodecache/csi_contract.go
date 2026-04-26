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

package nodecache

const (
	CSIDriverName = "node-cache.ai-models.deckhouse.io"

	CSIAttributeArtifactURI    = "ai.deckhouse.io/artifact-uri"
	CSIAttributeArtifactDigest = "ai.deckhouse.io/artifact-digest"
	CSIAttributeArtifactFamily = "ai.deckhouse.io/artifact-family"

	CSIEndpointEnv           = "AI_MODELS_NODE_CACHE_CSI_ENDPOINT"
	CSIContainerSocketPath   = "/csi/csi.sock"
	CSIKubeletPluginDir      = "/var/lib/kubelet/csi-plugins/" + CSIDriverName
	CSIKubeletSocketPath     = CSIKubeletPluginDir + "/csi.sock"
	CSIRegistrationDirectory = "/var/lib/kubelet/plugins_registry"
)
