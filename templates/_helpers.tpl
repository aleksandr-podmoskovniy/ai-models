{{- define "ai-models.moduleName" -}}
ai-models
{{- end -}}

{{- define "ai-models.fullname" -}}
ai-models
{{- end -}}

{{- define "ai-models.namespace" -}}
d8-ai-models
{{- end -}}

{{- define "ai-models.monitoringNamespace" -}}
d8-monitoring
{{- end -}}

{{- define "ai-models.priorityClassName" -}}
system-cluster-critical
{{- end -}}

{{- define "ai-models.vpaPolicyUpdateMode" -}}
{{- $kubeVersion := .Values.global.discovery.kubernetesVersion -}}
{{- if semverCompare ">=1.33.0" $kubeVersion -}}
InPlaceOrRecreate
{{- else -}}
Recreate
{{- end -}}
{{- end -}}

{{- define "ai-models.controllerResources" -}}
cpu: 50m
memory: 128Mi
{{- end -}}

{{- define "ai-models.publicationMaxConcurrentWorkers" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- default 4 (index $runtime "maxConcurrentWorkers") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerCPURequest" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $requests := (index $resources "requests") | default dict -}}
{{- default "1" (index $requests "cpu") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerMemoryRequest" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $requests := (index $resources "requests") | default dict -}}
{{- default "1Gi" (index $requests "memory") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerEphemeralStorageRequest" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $requests := (index $resources "requests") | default dict -}}
{{- default "1Gi" (index $requests "ephemeral-storage") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerCPULimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $limits := (index $resources "limits") | default dict -}}
{{- default "4" (index $limits "cpu") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerMemoryLimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $limits := (index $resources "limits") | default dict -}}
{{- default "2Gi" (index $limits "memory") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerEphemeralStorageLimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $limits := (index $resources "limits") | default dict -}}
{{- default "1Gi" (index $limits "ephemeral-storage") -}}
{{- end -}}

{{- define "ai-models.nodeCacheEnabled" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- if and (hasKey $nodeCache "enabled") (index $nodeCache "enabled") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "ai-models.nodeCacheMaxSize" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "200Gi" (index $nodeCache "maxSize") -}}
{{- end -}}

{{- define "ai-models.nodeCacheSharedVolumeSize" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "64Gi" (index $nodeCache "sharedVolumeSize") -}}
{{- end -}}

{{- define "ai-models.nodeCacheQuantityBytes" -}}
{{- $value := . | toString | trim -}}
{{- if not (regexMatch "^[0-9]+([KMGT]i?)?$" $value) -}}
  {{- fail (printf "aiModels.nodeCache quantity %q must be an integer optionally suffixed with Ki, Mi, Gi, Ti, K, M, G, or T" $value) -}}
{{- end -}}
{{- $suffix := regexFind "[A-Za-z]+$" $value -}}
{{- $number := int64 (regexReplaceAll "[A-Za-z]+$" $value "") -}}
{{- $multiplier := int64 1 -}}
{{- if eq $suffix "K" -}}
  {{- $multiplier = int64 1000 -}}
{{- else if eq $suffix "M" -}}
  {{- $multiplier = int64 1000000 -}}
{{- else if eq $suffix "G" -}}
  {{- $multiplier = int64 1000000000 -}}
{{- else if eq $suffix "T" -}}
  {{- $multiplier = int64 1000000000000 -}}
{{- else if eq $suffix "Ki" -}}
  {{- $multiplier = int64 1024 -}}
{{- else if eq $suffix "Mi" -}}
  {{- $multiplier = int64 1048576 -}}
{{- else if eq $suffix "Gi" -}}
  {{- $multiplier = int64 1073741824 -}}
{{- else if eq $suffix "Ti" -}}
  {{- $multiplier = int64 1099511627776 -}}
{{- end -}}
{{- mul $number $multiplier -}}
{{- end -}}

{{- define "ai-models.nodeCacheStorageClassName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "ai-models-node-cache" (index $nodeCache "storageClassName") -}}
{{- end -}}

{{- define "ai-models.nodeCacheVolumeGroupSetName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "ai-models-node-cache" (index $nodeCache "volumeGroupSetName") -}}
{{- end -}}

{{- define "ai-models.nodeCacheVolumeGroupNameOnNode" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "ai-models-cache" (index $nodeCache "volumeGroupNameOnNode") -}}
{{- end -}}

{{- define "ai-models.nodeCacheThinPoolName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- default "model-cache" (index $nodeCache "thinPoolName") -}}
{{- end -}}

{{- define "ai-models.nodeCacheNodeSelectorJSON" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- $selector := (index $nodeCache "nodeSelector") | default dict -}}
{{- toJson $selector -}}
{{- end -}}

{{- define "ai-models.nodeCacheBlockDeviceSelectorJSON" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
{{- $selector := (index $nodeCache "blockDeviceSelector") | default dict -}}
{{- toJson $selector -}}
{{- end -}}

{{- define "ai-models.nodeCacheRuntimeName" -}}
ai-models-node-cache-runtime
{{- end -}}

{{- define "ai-models.nodeCacheCSIDriverName" -}}
node-cache.ai-models.deckhouse.io
{{- end -}}

{{- define "ai-models.nodeCacheCSIRegistrarImage" -}}
{{- $kubernetesSemVer := semver .Values.global.discovery.kubernetesVersion -}}
{{- include "helm_lib_csi_image_with_common_fallback" (list . "csiNodeDriverRegistrar" $kubernetesSemVer) -}}
{{- end -}}

{{- define "ai-models.nodeCacheMaintenanceMaxUnusedAge" -}}
24h
{{- end -}}

{{- define "ai-models.nodeCacheMaintenanceScanInterval" -}}
5m
{{- end -}}

{{- define "ai-models.dmcrResources" -}}
cpu: 50m
memory: 128Mi
{{- end -}}

{{- define "ai-models.dmcrGarbageCollectionResources" -}}
cpu: 50m
memory: 64Mi
{{- end -}}

{{- define "ai-models.dmcrGCRequestLabelKey" -}}
ai.deckhouse.io/dmcr-gc-request
{{- end -}}

{{- define "ai-models.dmcrGCSchedule" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $dmcr := (index $moduleValues "dmcr") | default dict -}}
{{- $gc := (index $dmcr "gc") | default dict -}}
{{- if hasKey $gc "schedule" -}}
{{- index $gc "schedule" -}}
{{- else -}}
*/20 * * * *
{{- end -}}
{{- end -}}

{{- define "ai-models.serviceName" -}}
ai-models
{{- end -}}

{{- define "ai-models.serviceAccountName" -}}
ai-models
{{- end -}}

{{- define "ai-models.dmcrName" -}}
dmcr
{{- end -}}

{{- define "ai-models.dmcrServiceAccountName" -}}
{{- include "ai-models.dmcrName" . -}}
{{- end -}}

{{- define "ai-models.dmcrConfigMapName" -}}
ai-models-dmcr
{{- end -}}

{{- define "ai-models.controllerName" -}}
ai-models-controller
{{- end -}}

{{- define "ai-models.controllerServiceAccountName" -}}
{{- include "ai-models.controllerName" . -}}
{{- end -}}

{{- define "ai-models.controllerServiceName" -}}
{{- include "ai-models.controllerName" . -}}
{{- end -}}

{{- define "ai-models.controllerMetricsServiceName" -}}
{{- include "ai-models.controllerServiceName" . -}}
{{- end -}}

{{- define "ai-models.uploadGatewayName" -}}
ai-models-upload-gateway
{{- end -}}

{{- define "ai-models.uploadGatewayServiceAccountName" -}}
{{- include "ai-models.uploadGatewayName" . -}}
{{- end -}}

{{- define "ai-models.uploadGatewayServiceName" -}}
{{- include "ai-models.uploadGatewayName" . -}}
{{- end -}}

{{- define "ai-models.configMapName" -}}
ai-models-runtime
{{- end -}}

{{- define "ai-models.dmcrAuthSecretName" -}}
ai-models-dmcr-auth
{{- end -}}

{{- define "ai-models.dmcrWriteAuthSecretName" -}}
ai-models-dmcr-auth-write
{{- end -}}

{{- define "ai-models.dmcrReadAuthSecretName" -}}
ai-models-dmcr-auth-read
{{- end -}}

{{- define "ai-models.dmcrTLSSecretName" -}}
ai-models-dmcr-tls
{{- end -}}

{{- define "ai-models.dmcrCASecretName" -}}
ai-models-dmcr-ca
{{- end -}}

{{- define "ai-models.dmcrSecretRestartChecksumAnnotationKey" -}}
ai.deckhouse.io/dmcr-pod-secret-checksum
{{- end -}}

{{- define "ai-models.dmcrAuthSecretRestartChecksum" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $dmcr := (index $internal "dmcr") | default dict -}}
{{- $auth := (index $dmcr "auth") | default dict -}}
{{- $writePassword := (index $auth "writePassword") | default "" | trim -}}
{{- $readPassword := (index $auth "readPassword") | default "" | trim -}}
{{- $writeHtpasswd := (index $auth "writeHtpasswd") | default "" | trim -}}
{{- $readHtpasswd := (index $auth "readHtpasswd") | default "" | trim -}}
{{- $salt := (index $auth "salt") | default "" | trim -}}
{{- if or $salt $writePassword $readPassword $writeHtpasswd $readHtpasswd -}}
{{- printf "%s\n%s\n%s\n%s\n%s\n%s\n%s" (sha256sum $salt) (include "ai-models.dmcrPasswordChecksum" $writePassword | trim) (include "ai-models.dmcrPasswordChecksum" $readPassword | trim) (sha256sum $writeHtpasswd) (sha256sum $readHtpasswd) (include "ai-models.dmcrWriteAuthUsername" . | trim) (include "ai-models.dmcrReadAuthUsername" . | trim) | sha256sum -}}
{{- else -}}
auth/bootstrap
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrTLSSecretRestartChecksum" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $dmcr := (index $internal "dmcr") | default dict -}}
{{- $cert := (index $dmcr "cert") | default dict -}}
{{- $caCert := (index $cert "ca") | default "" | trim -}}
{{- $tlsCert := (index $cert "crt") | default "" | trim -}}
{{- $tlsKey := (index $cert "key") | default "" | trim -}}
{{- if or $caCert $tlsCert $tlsKey -}}
{{- printf "%s\n%s\n%s" (sha256sum $caCert) (sha256sum $tlsCert) (sha256sum $tlsKey) | sha256sum -}}
{{- else -}}
tls/bootstrap
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrPodSecretChecksum" -}}
{{- printf "%s\n%s" (include "ai-models.dmcrAuthSecretRestartChecksum" . | trim) (include "ai-models.dmcrTLSSecretRestartChecksum" . | trim) | sha256sum -}}
{{- end -}}

{{- define "ai-models.artifactsPlatformCASecretName" -}}
ai-models-artifacts-platform-ca
{{- end -}}

{{- define "ai-models.discoveredDexCA" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- default "" (index $internal "discoveredDexCA") -}}
{{- end -}}

{{- define "ai-models.globalCustomCertificateCA" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $customCertificateData := (index $internal "customCertificateData") | default dict -}}
{{- default "" (index $customCertificateData "ca.crt") -}}
{{- end -}}

{{- define "ai-models.platformTrustCA" -}}
{{- $discoveredDexCA := include "ai-models.discoveredDexCA" . | trim -}}
{{- if $discoveredDexCA -}}
{{- $discoveredDexCA -}}
{{- else -}}
{{- include "ai-models.globalCustomCertificateCA" . | trim -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.hasPlatformTrustCA" -}}
{{- if (include "ai-models.platformTrustCA" . | trim) -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.registrySecretName" -}}
{{- printf "%s-module-registry" .Chart.Name -}}
{{- end -}}

{{- define "ai-models.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ai-models.fullname" . }}
app.kubernetes.io/part-of: {{ include "ai-models.moduleName" . }}
{{- end -}}

{{- define "ai-models.registryDockerConfig" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $moduleRegistry := (index $moduleValues "registry") | default dict -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $modulesImages := (index $globalValues "modulesImages") | default dict -}}
{{- $globalRegistry := (index $modulesImages "registry") | default dict -}}
{{- default ((index $globalRegistry "dockercfg") | default "") ((index $moduleRegistry "dockercfg") | default "") -}}
{{- end -}}

{{- define "ai-models.hasRegistrySecret" -}}
{{- if (include "ai-models.registryDockerConfig" . | trim) -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.isEnabled" -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $enabledModules := (index $globalValues "enabledModules") | default list -}}
{{- if has (include "ai-models.moduleName" .) $enabledModules -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.publicHost" -}}
{{- include "helm_lib_module_public_domain" (list . "ai-models") -}}
{{- end -}}

{{- define "ai-models.uploadPublicHost" -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $modulesValues := (index $globalValues "modules") | default dict -}}
{{- if (default "" (index $modulesValues "publicDomainTemplate")) -}}
{{- include "ai-models.publicHost" . -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.uploadIngressTLSSecretName" -}}
{{- if and (include "ai-models.uploadPublicHost" . | trim) (include "helm_lib_module_https_ingress_tls_enabled" .) -}}
{{- include "helm_lib_module_https_secret_name" (list . "ingress-tls") -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.clusterDomain" -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $clusterConfiguration := (index $globalValues "clusterConfiguration") | default dict -}}
{{- $discovery := (index $globalValues "discovery") | default dict -}}
{{- default (index $clusterConfiguration "clusterDomain") (index $discovery "clusterDomain") -}}
{{- end -}}

{{- define "ai-models.controllerMetricsPort" -}}
8080
{{- end -}}

{{- define "ai-models.controllerHealthPort" -}}
8081
{{- end -}}

{{- define "ai-models.uploadGatewayPort" -}}
8444
{{- end -}}

{{- define "ai-models.controllerWebhookPort" -}}
9443
{{- end -}}

{{- define "ai-models.controllerWebhookTLSSecretName" -}}
ai-models-controller-webhook-tls
{{- end -}}

{{- define "ai-models.controllerLeaderElectionID" -}}
ai-models-controller.deckhouse.io
{{- end -}}

{{- define "ai-models.controllerReplicas" -}}
{{- if (include "helm_lib_ha_enabled" .) -}}
2
{{- else -}}
1
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsBucket" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "bucket") -}}
{{- end -}}

{{- define "ai-models.artifactsEndpoint" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "endpoint") -}}
{{- end -}}

{{- define "ai-models.artifactsEndpointHost" -}}
{{- $endpoint := include "ai-models.artifactsEndpoint" . | trim -}}
{{- trimPrefix "http://" (trimPrefix "https://" $endpoint) -}}
{{- end -}}

{{- define "ai-models.artifactsEndpointSecure" -}}
{{- $endpoint := include "ai-models.artifactsEndpoint" . | trim -}}
{{- if hasPrefix "http://" $endpoint -}}
false
{{- else -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsRegion" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "us-east-1" (index $artifacts "region") -}}
{{- end -}}

{{- define "ai-models.sourceFetchMode" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- lower (default "Direct" (index $artifacts "sourceFetchMode")) -}}
{{- end -}}

{{- define "ai-models.artifactsCredentialsSecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "credentialsSecretName") -}}
{{- end -}}

{{- define "ai-models.artifactsCASecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "caSecretName") -}}
{{- end -}}

{{- define "ai-models.artifactsCapacityLimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "capacityLimit") -}}
{{- end -}}

{{- define "ai-models.artifactsSyncedCredentialsSecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $artifacts := (index $internal "artifacts") | default dict -}}
{{- default "ai-models-artifacts" (index $artifacts "syncedCredentialsSecretName") -}}
{{- end -}}

{{- define "ai-models.artifactsMountedCASecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $artifacts := (index $internal "artifacts") | default dict -}}
{{- $mounted := default "" (index $artifacts "mountedCASecretName") -}}
{{- if $mounted -}}
{{- $mounted -}}
{{- else if (include "ai-models.hasPlatformTrustCA" . | trim) -}}
{{- include "ai-models.artifactsPlatformCASecretName" . -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsResolvedSecretName" -}}
{{- include "ai-models.artifactsSyncedCredentialsSecretName" . -}}
{{- end -}}

{{- define "ai-models.artifactsUsePathStyle" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- if hasKey $artifacts "usePathStyle" -}}
{{- ternary "path" "virtual" (index $artifacts "usePathStyle") -}}
{{- else -}}
path
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsUsePathStyleBool" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- if hasKey $artifacts "usePathStyle" -}}
{{- ternary "true" "false" (index $artifacts "usePathStyle") -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsIgnoreTLS" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- if and (hasKey $artifacts "insecure") (index $artifacts "insecure") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrRootDirectory" -}}
/dmcr
{{- end -}}

{{- define "ai-models.dmcrServiceHost" -}}
{{- printf "%s.%s.svc.%s" (include "ai-models.dmcrName" .) (include "ai-models.namespace" .) (include "ai-models.clusterDomain" .) -}}
{{- end -}}

{{- define "ai-models.dmcrDirectUploadPort" -}}
5443
{{- end -}}

{{- define "ai-models.dmcrDirectUploadEndpoint" -}}
{{- printf "https://%s:%s" (include "ai-models.dmcrServiceHost" .) (include "ai-models.dmcrDirectUploadPort" .) -}}
{{- end -}}

{{- define "ai-models.dmcrRepositoryPrefix" -}}
{{- printf "%s/ai-models" (include "ai-models.dmcrServiceHost" .) -}}
{{- end -}}

{{- define "ai-models.dmcrWriteAuthUsername" -}}
ai-models
{{- end -}}

{{- define "ai-models.dmcrReadAuthUsername" -}}
ai-models-reader
{{- end -}}

{{- define "ai-models.dmcrDockerConfigJSON" -}}
{{- $root := index . 0 -}}
{{- $host := include "ai-models.dmcrServiceHost" $root -}}
{{- $username := index . 1 -}}
{{- $password := index . 2 -}}
{{- $auth := printf "%s:%s" $username $password | b64enc -}}
{{- dict "auths" (dict $host (dict "username" $username "password" $password "auth" $auth)) | toJson -}}
{{- end -}}

{{- define "ai-models.dmcrPasswordChecksum" -}}
{{- sha256sum (trim (toString .)) -}}
{{- end -}}

{{- define "ai-models.hasAPI" -}}
{{- $context := index . 0 -}}
{{- $apiVersion := index . 1 -}}
{{- $globalValues := (index $context.Values "global") | default dict -}}
{{- $discovery := (index $globalValues "discovery") | default dict -}}
{{- $apiVersions := (index $discovery "apiVersions") | default list -}}
{{- if or ($context.Capabilities.APIVersions.Has $apiVersion) (has $apiVersion $apiVersions) -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.validate" -}}
{{- if (include "ai-models.isEnabled" .) -}}
  {{- $globalValues := (index .Values "global") | default dict -}}
  {{- $enabledModules := (index $globalValues "enabledModules") | default list -}}
  {{- $modulesValues := (index $globalValues "modules") | default dict -}}
  {{- $httpsMode := include "helm_lib_module_https_mode" . -}}
  {{- $publicDomainTemplate := default "" (index $modulesValues "publicDomainTemplate") -}}
  {{- if not $publicDomainTemplate -}}
    {{- fail "global.modules.publicDomainTemplate is required for ai-models UI ingress" -}}
  {{- end -}}
  {{- if eq $httpsMode "Disabled" -}}
    {{- fail "ai-models requires HTTPS enabled via global.modules.https" -}}
  {{- end -}}
  {{- if not (include "ai-models.artifactsBucket" . | trim) -}}
    {{- fail "ai-models.artifacts.bucket is required" -}}
  {{- end -}}
  {{- if not (include "ai-models.artifactsEndpoint" . | trim) -}}
    {{- fail "ai-models.artifacts.endpoint is required" -}}
  {{- end -}}
  {{- if not (include "ai-models.artifactsCredentialsSecretName" . | trim) -}}
    {{- fail "ai-models.artifacts.credentialsSecretName is required" -}}
  {{- end -}}
  {{- $moduleValues := (index .Values "aiModels") | default dict -}}
  {{- $nodeCache := (index $moduleValues "nodeCache") | default dict -}}
  {{- if and (hasKey $nodeCache "enabled") (index $nodeCache "enabled") -}}
    {{- range $module := list "sds-node-configurator-crd" "sds-node-configurator" "sds-local-volume-crd" "sds-local-volume" -}}
      {{- if not (has $module $enabledModules) -}}
        {{- fail (printf "aiModels.nodeCache.enabled requires global.enabledModules to include %s" $module) -}}
      {{- end -}}
    {{- end -}}
    {{- $nodeSelector := (index $nodeCache "nodeSelector") | default dict -}}
    {{- if eq (len $nodeSelector) 0 -}}
      {{- fail "aiModels.nodeCache.nodeSelector must not be empty when nodeCache is enabled" -}}
    {{- end -}}
    {{- $blockDeviceSelector := (index $nodeCache "blockDeviceSelector") | default dict -}}
    {{- if eq (len $blockDeviceSelector) 0 -}}
      {{- fail "aiModels.nodeCache.blockDeviceSelector must not be empty when nodeCache is enabled" -}}
    {{- end -}}
    {{- $maxSizeBytes := include "ai-models.nodeCacheQuantityBytes" (include "ai-models.nodeCacheMaxSize" .) | int64 -}}
    {{- $sharedVolumeSizeBytes := include "ai-models.nodeCacheQuantityBytes" (include "ai-models.nodeCacheSharedVolumeSize" .) | int64 -}}
    {{- if gt $sharedVolumeSizeBytes $maxSizeBytes -}}
      {{- fail "aiModels.nodeCache.sharedVolumeSize must not be greater than aiModels.nodeCache.maxSize" -}}
    {{- end -}}
    {{- if not (include "ai-models.nodeCacheCSIRegistrarImage" . | trim) -}}
      {{- fail "aiModels.nodeCache.enabled requires csiNodeDriverRegistrar image digest in global.modulesImages" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
