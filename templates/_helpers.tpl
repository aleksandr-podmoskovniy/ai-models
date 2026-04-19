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
{{- default 1 (index $runtime "maxConcurrentWorkers") -}}
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
{{- default "8Gi" (index $requests "memory") -}}
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
{{- default "16Gi" (index $limits "memory") -}}
{{- end -}}

{{- define "ai-models.publicationWorkerEphemeralStorageLimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $resources := (index $runtime "resources") | default dict -}}
{{- $limits := (index $resources "limits") | default dict -}}
{{- default "1Gi" (index $limits "ephemeral-storage") -}}
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

{{- define "ai-models.dmcrGCSwitchAnnotationKey" -}}
ai.deckhouse.io/dmcr-gc-switch
{{- end -}}

{{- define "ai-models.dmcrGCDoneAnnotationKey" -}}
ai.deckhouse.io/dmcr-gc-done
{{- end -}}

{{- define "ai-models.dmcrGarbageCollectionModeEnabled" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $dmcr := (index $internal "dmcr") | default dict -}}
{{- if and (hasKey $dmcr "garbageCollectionModeEnabled") (index $dmcr "garbageCollectionModeEnabled") -}}
true
{{- else -}}
false
{{- end -}}
{{- end -}}

{{- define "ai-models.controlPlaneNodeSelector" -}}
nodeSelector:
  node-role.kubernetes.io/control-plane: ""
{{- end -}}

{{- define "ai-models.systemNodeSelector" -}}
{{- $discovery := (.Values.global.discovery | default dict) -}}
{{- $roles := ($discovery.d8SpecificNodeCountByRole | default dict) -}}
{{- if gt (int (default 0 (index $roles "system"))) 0 -}}
nodeSelector:
  node-role.deckhouse.io/system: ""
{{- else -}}
{{- include "ai-models.controlPlaneNodeSelector" . -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.controlPlaneTolerations" -}}
tolerations:
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
  - key: node-role.kubernetes.io/master
    operator: Exists
    effect: NoSchedule
{{- end -}}

{{- define "ai-models.systemTolerations" -}}
tolerations:
  - key: node-role.deckhouse.io/system
    operator: Exists
    effect: NoSchedule
  - key: node-role.kubernetes.io/control-plane
    operator: Exists
    effect: NoSchedule
  - key: node-role.kubernetes.io/master
    operator: Exists
    effect: NoSchedule
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

{{- define "ai-models.controllerUploadPort" -}}
8444
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

{{- define "ai-models.huggingFaceAcquisitionMode" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- lower (default "Mirror" (index $artifacts "huggingFaceAcquisitionMode")) -}}
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

{{- define "ai-models.dmcrWriteAuthPassword" -}}
{{- $namespace := include "ai-models.namespace" . -}}
{{- $authSecretName := include "ai-models.dmcrAuthSecretName" . -}}
{{- $authSecret := lookup "v1" "Secret" $namespace $authSecretName -}}
{{- $authData := (get (default (dict) $authSecret) "data" | default (dict)) -}}
{{- $serverPassword := (get $authData "write.password" | default "" | b64dec) -}}
{{- $secretName := include "ai-models.dmcrWriteAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" $namespace $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingPassword := (get $existingData "password" | default "" | b64dec) -}}
{{- $generatedPassword := printf "A1a%s" (randAlphaNum 37) -}}
{{- coalesce $serverPassword $existingPassword $generatedPassword -}}
{{- end -}}

{{- define "ai-models.dmcrReadAuthPassword" -}}
{{- $namespace := include "ai-models.namespace" . -}}
{{- $authSecretName := include "ai-models.dmcrAuthSecretName" . -}}
{{- $authSecret := lookup "v1" "Secret" $namespace $authSecretName -}}
{{- $authData := (get (default (dict) $authSecret) "data" | default (dict)) -}}
{{- $serverPassword := (get $authData "read.password" | default "" | b64dec) -}}
{{- $secretName := include "ai-models.dmcrReadAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" $namespace $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingPassword := (get $existingData "password" | default "" | b64dec) -}}
{{- $generatedPassword := printf "A1a%s" (randAlphaNum 37) -}}
{{- coalesce $serverPassword $existingPassword $generatedPassword -}}
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

{{- define "ai-models.dmcrWriteHTPasswdEntry" -}}
{{- $root := index . 0 -}}
{{- $desiredPassword := index . 1 | trim -}}
{{- $desiredChecksum := include "ai-models.dmcrPasswordChecksum" $desiredPassword | trim -}}
{{- $secretName := include "ai-models.dmcrAuthSecretName" $root -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" $root) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingEntry := (get $existingData "write.htpasswd" | default "" | b64dec) -}}
{{- $storedPassword := (get $existingData "write.password" | default "" | b64dec) -}}
{{- $storedChecksum := (get $existingData "write.htpasswd.checksum" | default "" | b64dec) -}}
{{- if and $existingEntry $storedPassword $storedChecksum (eq ($storedPassword | trim) $desiredPassword) (eq ($storedChecksum | trim) $desiredChecksum) -}}
{{- $existingEntry -}}
{{- else -}}
{{- htpasswd (include "ai-models.dmcrWriteAuthUsername" $root | trim) $desiredPassword -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrReadHTPasswdEntry" -}}
{{- $root := index . 0 -}}
{{- $desiredPassword := index . 1 | trim -}}
{{- $desiredChecksum := include "ai-models.dmcrPasswordChecksum" $desiredPassword | trim -}}
{{- $secretName := include "ai-models.dmcrAuthSecretName" $root -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" $root) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingEntry := (get $existingData "read.htpasswd" | default "" | b64dec) -}}
{{- $storedPassword := (get $existingData "read.password" | default "" | b64dec) -}}
{{- $storedChecksum := (get $existingData "read.htpasswd.checksum" | default "" | b64dec) -}}
{{- if and $existingEntry $storedPassword $storedChecksum (eq ($storedPassword | trim) $desiredPassword) (eq ($storedChecksum | trim) $desiredChecksum) -}}
{{- $existingEntry -}}
{{- else -}}
{{- htpasswd (include "ai-models.dmcrReadAuthUsername" $root | trim) $desiredPassword -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrHTTPSalt" -}}
{{- $secretName := include "ai-models.dmcrAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingSalt := (get $existingData "salt" | default "" | b64dec) -}}
{{- $generatedSalt := randAlphaNum 64 -}}
{{- coalesce $existingSalt $generatedSalt -}}
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
{{- end -}}
{{- end -}}
