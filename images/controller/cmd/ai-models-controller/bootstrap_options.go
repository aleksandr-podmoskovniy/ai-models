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

package main

import (
	"time"

	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/modeldelivery"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/sourceworker"
	"github.com/deckhouse/ai-models/controller/internal/adapters/k8s/storageprojection"
	modelpackoci "github.com/deckhouse/ai-models/controller/internal/adapters/modelpack/oci"
	uploadstagings3 "github.com/deckhouse/ai-models/controller/internal/adapters/uploadstaging/s3"
	"github.com/deckhouse/ai-models/controller/internal/bootstrap"
	"github.com/deckhouse/ai-models/controller/internal/cmdsupport"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogcleanup"
	"github.com/deckhouse/ai-models/controller/internal/controllers/catalogstatus"
	"github.com/deckhouse/ai-models/controller/internal/controllers/nodecacheruntime"
	"github.com/deckhouse/ai-models/controller/internal/controllers/nodecachesubstrate"
	"github.com/deckhouse/ai-models/controller/internal/controllers/workloaddelivery"
	"github.com/deckhouse/ai-models/controller/internal/dataplane/artifactcleanup"
	"github.com/deckhouse/ai-models/controller/internal/nodecache"
	corev1 "k8s.io/api/core/v1"
)

func (c managerConfig) objectStorageOptions() storageprojection.Options {
	return storageprojection.Options{
		Bucket:                c.ArtifactsBucket,
		EndpointURL:           c.ArtifactsS3Endpoint,
		Region:                c.ArtifactsS3Region,
		UsePathStyle:          c.ArtifactsS3UsePathStyle,
		Insecure:              c.ArtifactsS3IgnoreTLS,
		CredentialsSecretName: c.ArtifactsCredentialsSecretName,
		CASecretName:          c.ArtifactsCASecretName,
	}
}

func (c managerConfig) artifactCleaner() artifactcleanup.Cleaner {
	return artifactcleanup.Cleaner{
		Remover: modelpackoci.New(),
		ObjectStorage: func() (artifactcleanup.ObjectStorageRemover, error) {
			return uploadstagings3.New(cmdsupport.UploadStagingS3ConfigFromEnv())
		},
		ObjectStorageBucket: c.ArtifactsBucket,
		RegistryAuth:        cmdsupport.RegistryAuthFromEnvWithInsecure(c.PublicationOCIInsecure),
	}
}

func (c managerConfig) bootstrapOptions(resources corev1.ResourceRequirements) bootstrap.Options {
	artifactsObjectStorage := c.objectStorageOptions()
	nodeSelectorLabels, _ := parseMatchLabelsJSON(c.NodeCacheNodeSelectorJSON)
	blockDeviceSelectorLabels, _ := parseMatchLabelsJSON(c.NodeCacheBlockDeviceJSON)
	managedDeliveryNodeSelector := managedNodeCacheDeliverySelector(nodeSelectorLabels)

	return bootstrap.Options{
		Cleanup: c.cleanupOptions(),
		PublicationRuntime: c.publicationRuntimeOptions(
			resources,
			artifactsObjectStorage,
		),
		NodeCacheRuntime:   c.nodeCacheRuntimeOptions(nodeSelectorLabels),
		NodeCacheSubstrate: c.nodeCacheSubstrateOptions(nodeSelectorLabels, blockDeviceSelectorLabels),
		WorkloadDelivery:   c.workloadDeliveryOptions(managedDeliveryNodeSelector),
		Runtime: bootstrap.RuntimeOptions{
			MetricsBindAddress:      c.MetricsBindAddress,
			HealthProbeBindAddress:  c.HealthProbeBindAddress,
			LeaderElection:          c.LeaderElect,
			LeaderElectionID:        c.LeaderElectionID,
			LeaderElectionNamespace: c.LeaderElectionNamespace,
		},
	}
}

func (c managerConfig) cleanupOptions() catalogcleanup.Options {
	return catalogcleanup.Options{
		Cleanup: catalogcleanup.CleanupOptions{
			Namespace: c.CleanupNamespace,
			Cleaner:   c.artifactCleaner(),
		},
		RuntimeNamespace: c.PublicationWorkerNamespace,
		RequeueAfter:     5 * time.Second,
	}
}

func (c managerConfig) publicationRuntimeOptions(
	resources corev1.ResourceRequirements,
	artifactsObjectStorage storageprojection.Options,
) catalogstatus.Options {
	return catalogstatus.Options{
		RuntimeLogFormat: c.LogFormat,
		RuntimeLogLevel:  c.LogLevel,
		Runtime: sourceworker.RuntimeOptions{
			Namespace:               c.PublicationWorkerNamespace,
			Image:                   c.PublicationWorkerImage,
			ImagePullSecretName:     c.PublicationWorkerImagePullSecretName,
			ServiceAccountName:      c.PublicationWorkerServiceAccount,
			OCIRepositoryPrefix:     c.PublicationOCIRepositoryPrefix,
			OCIInsecure:             c.PublicationOCIInsecure,
			OCIRegistrySecretName:   c.PublicationOCISecretName,
			OCIRegistryCASecretName: c.PublicationOCICASecretName,
			OCIDirectUploadEndpoint: c.PublicationOCIDirectUploadEndpoint,
			ObjectStorage:           artifactsObjectStorage,
			SourceFetch:             c.PublicationSourceFetchMode,
			Resources:               resources,
		},
		MaxConcurrentWorkers:  c.PublicationMaxConcurrentWorkers,
		CleanupStateNamespace: c.CleanupNamespace,
		UploadGateway: catalogstatus.UploadGatewayOptions{
			ServiceName: c.UploadServiceName,
			PublicHost:  c.UploadPublicHost,
		},
	}
}

func (c managerConfig) nodeCacheRuntimeOptions(nodeSelectorLabels map[string]string) nodecacheruntime.Options {
	return nodecacheruntime.Options{
		Enabled:             c.NodeCacheEnabled,
		Namespace:           c.CleanupNamespace,
		RuntimeImage:        c.NodeCacheRuntimeImage,
		CSIRegistrarImage:   c.NodeCacheCSIRegistrarImage,
		ImagePullSecretName: c.PublicationWorkerImagePullSecretName,
		ServiceAccountName:  nodecache.RuntimeServiceAccount,
		StorageClassName:    c.NodeCacheStorageClassName,
		SharedVolumeSize:    c.NodeCacheSharedVolumeSize,
		MaxTotalSize:        c.NodeCacheSharedVolumeSize,
		MaxUnusedAge:        nodecache.DefaultMaxUnusedAge.String(),
		ScanInterval:        nodecache.DefaultMaintenancePeriod.String(),
		OCIInsecure:         c.PublicationOCIInsecure,
		OCIAuthSecretName:   defaultDMCRReadAuthSecretName,
		OCIRegistryCASecret: c.PublicationOCICASecretName,
		NodeSelectorLabels:  nodeSelectorLabels,
	}
}

func (c managerConfig) nodeCacheSubstrateOptions(
	nodeSelectorLabels map[string]string,
	blockDeviceSelectorLabels map[string]string,
) nodecachesubstrate.Options {
	return nodecachesubstrate.Options{
		Enabled:                c.NodeCacheEnabled,
		MaxSize:                c.NodeCacheMaxSize,
		StorageClassName:       c.NodeCacheStorageClassName,
		VolumeGroupSetName:     c.NodeCacheVolumeGroupSetName,
		VolumeGroupNameOnNode:  c.NodeCacheVolumeGroupNameOnNode,
		ThinPoolName:           c.NodeCacheThinPoolName,
		NodeSelectorLabels:     nodeSelectorLabels,
		BlockDeviceMatchLabels: blockDeviceSelectorLabels,
	}
}

func (c managerConfig) workloadDeliveryOptions(managedDeliveryNodeSelector map[string]string) workloaddelivery.Options {
	return workloaddelivery.Options{
		Service: modeldelivery.ServiceOptions{
			Render: modeldelivery.Options{
				RuntimeImage:   c.PublicationWorkerImage,
				LogFormat:      c.LogFormat,
				LogLevel:       c.LogLevel,
				OCIInsecure:    c.PublicationOCIInsecure,
				CacheMountPath: modeldelivery.DefaultCacheMountPath,
			},
			ManagedCache: modeldelivery.ManagedCacheOptions{
				Enabled:      c.NodeCacheEnabled,
				VolumeName:   modeldelivery.DefaultManagedCacheName,
				NodeSelector: managedDeliveryNodeSelector,
			},
			RegistrySourceNamespace:      cmdsupport.FallbackString(c.PublicationWorkerNamespace, c.CleanupNamespace),
			RegistrySourceAuthSecretName: defaultDMCRReadAuthSecretName,
			RegistrySourceCASecretName:   c.PublicationOCICASecretName,
			RuntimeImagePullSecretName: cmdsupport.FallbackString(
				c.WorkloadDeliveryRuntimeImagePullSecretName,
				c.PublicationWorkerImagePullSecretName,
			),
		},
	}
}

func managedNodeCacheDeliverySelector(nodeSelectorLabels map[string]string) map[string]string {
	if len(nodeSelectorLabels) == 0 {
		return nil
	}
	selector := make(map[string]string, len(nodeSelectorLabels)+1)
	for key, value := range nodeSelectorLabels {
		selector[key] = value
	}
	selector[nodecache.RuntimeReadyNodeLabelKey] = nodecache.RuntimeReadyNodeLabelValue
	return selector
}
