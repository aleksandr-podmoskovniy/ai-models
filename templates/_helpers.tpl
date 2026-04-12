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

{{- define "ai-models.publicationWorkVolumeType" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $workVolume := (index $runtime "workVolume") | default dict -}}
{{- default "EmptyDir" (index $workVolume "type") -}}
{{- end -}}

{{- define "ai-models.publicationWorkVolumeEmptyDirSizeLimit" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $workVolume := (index $runtime "workVolume") | default dict -}}
{{- $emptyDir := (index $workVolume "emptyDir") | default dict -}}
{{- default "50Gi" (index $emptyDir "sizeLimit") -}}
{{- end -}}

{{- define "ai-models.publicationWorkVolumePVCName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $workVolume := (index $runtime "workVolume") | default dict -}}
{{- $pvc := (index $workVolume "persistentVolumeClaim") | default dict -}}
{{- default "ai-models-publication-work" (index $pvc "name") -}}
{{- end -}}

{{- define "ai-models.publicationWorkVolumePVCStorageClassName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $workVolume := (index $runtime "workVolume") | default dict -}}
{{- $pvc := (index $workVolume "persistentVolumeClaim") | default dict -}}
{{- default "" (index $pvc "storageClassName") -}}
{{- end -}}

{{- define "ai-models.publicationWorkVolumePVCStorageSize" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $internal := (index $moduleValues "internal") | default dict -}}
{{- $runtime := (index $internal "publicationRuntime") | default dict -}}
{{- $workVolume := (index $runtime "workVolume") | default dict -}}
{{- $pvc := (index $workVolume "persistentVolumeClaim") | default dict -}}
{{- default "50Gi" (index $pvc "storageSize") -}}
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
{{- default "50Gi" (index $requests "ephemeral-storage") -}}
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
{{- default "50Gi" (index $limits "ephemeral-storage") -}}
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
ai-models.deckhouse.io/dmcr-gc-request
{{- end -}}

{{- define "ai-models.dmcrGCSwitchAnnotationKey" -}}
ai-models.deckhouse.io/dmcr-gc-switch
{{- end -}}

{{- define "ai-models.dmcrGCDoneAnnotationKey" -}}
ai-models.deckhouse.io/dmcr-gc-done
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

{{- define "ai-models.postgresqlSecretName" -}}
ai-models-postgresql
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

{{- define "ai-models.backendCryptoSecretName" -}}
ai-models-backend-crypto
{{- end -}}

{{- define "ai-models.backendAuthSecretName" -}}
ai-models-backend-auth
{{- end -}}

{{- define "ai-models.backendTrustCASecretName" -}}
ai-models-backend-trust-ca
{{- end -}}

{{- define "ai-models.backendAuthOIDCAlembicVersionTable" -}}
alembic_version_auth
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

{{- define "ai-models.dexClientName" -}}
ai-models
{{- end -}}

{{- define "ai-models.dexClientSecretName" -}}
{{- printf "dex-client-%s" (include "ai-models.dexClientName" .) -}}
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

{{- define "ai-models.backendPort" -}}
5000
{{- end -}}

{{- define "ai-models.backendInternalURL" -}}
{{- printf "http://%s.%s.svc.%s" (include "ai-models.serviceName" .) (include "ai-models.namespace" .) (include "ai-models.clusterDomain" .) -}}
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

{{- define "ai-models.backendMetricsDir" -}}
/tmp/ai-models-metrics
{{- end -}}

{{- define "ai-models.authSSOProviderDisplayName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $auth := (index $moduleValues "auth") | default dict -}}
{{- $sso := (index $auth "sso") | default dict -}}
{{- default "Login with DKP SSO" (index $sso "providerDisplayName") -}}
{{- end -}}

{{- define "ai-models.authSSOAutomaticLoginRedirect" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $auth := (index $moduleValues "auth") | default dict -}}
{{- $sso := (index $auth "sso") | default dict -}}
{{- if hasKey $sso "automaticLoginRedirect" -}}
{{- ternary "true" "false" (index $sso "automaticLoginRedirect") -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.authSSOAllowedGroupsCSV" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $auth := (index $moduleValues "auth") | default dict -}}
{{- $sso := (index $auth "sso") | default dict -}}
{{- $groups := (index $sso "allowedGroups") | default (list "admins") -}}
{{- $result := list -}}
{{- range $group := $groups -}}
  {{- $clean := $group | toString | trim -}}
  {{- if $clean -}}
    {{- $result = append $result $clean -}}
  {{- end -}}
{{- end -}}
{{- join "," $result -}}
{{- end -}}

{{- define "ai-models.authSSOAdminGroupsCSV" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $auth := (index $moduleValues "auth") | default dict -}}
{{- $sso := (index $auth "sso") | default dict -}}
{{- $groups := (index $sso "adminGroups") | default (list "admins") -}}
{{- $result := list -}}
{{- range $group := $groups -}}
  {{- $clean := $group | toString | trim -}}
  {{- if $clean -}}
    {{- $result = append $result $clean -}}
  {{- end -}}
{{- end -}}
{{- join "," $result -}}
{{- end -}}

{{- define "ai-models.authSSODexClientAllowedGroupsYAML" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $auth := (index $moduleValues "auth") | default dict -}}
{{- $sso := (index $auth "sso") | default dict -}}
{{- $allowed := (index $sso "allowedGroups") | default (list "admins") -}}
{{- $admins := (index $sso "adminGroups") | default (list "admins") -}}
{{- $seen := dict -}}
{{- $result := list -}}
{{- range $group := concat $allowed $admins -}}
  {{- $clean := $group | toString | trim -}}
  {{- if and $clean (not (hasKey $seen $clean)) -}}
    {{- $_ := set $seen $clean true -}}
    {{- $result = append $result $clean -}}
  {{- end -}}
{{- end -}}
{{- toYaml $result -}}
{{- end -}}

{{- define "ai-models.dexClientID" -}}
{{- printf "dex-client-%s@%s" (include "ai-models.dexClientName" .) (include "ai-models.namespace" .) -}}
{{- end -}}

{{- define "ai-models.dexDiscoveryURL" -}}
{{- printf "https://%s/.well-known/openid-configuration" (include "helm_lib_module_public_domain" (list . "dex")) -}}
{{- end -}}

{{- define "ai-models.dexRedirectURI" -}}
{{- printf "https://%s/callback" (include "ai-models.publicHost" .) -}}
{{- end -}}

{{- define "ai-models.backendAllowedHosts" -}}
{{- $hosts := list -}}
{{- $publicHost := include "ai-models.publicHost" . | trim -}}
{{- $namespace := include "ai-models.namespace" . -}}
{{- $serviceName := include "ai-models.serviceName" . -}}
{{- $clusterDomain := include "ai-models.clusterDomain" . -}}
{{- range $host := list
  $publicHost
  (printf "%s:*" $publicHost)
  $serviceName
  (printf "%s:*" $serviceName)
  (printf "%s.%s" $serviceName $namespace)
  (printf "%s.%s:*" $serviceName $namespace)
  (printf "%s.%s.svc" $serviceName $namespace)
  (printf "%s.%s.svc:*" $serviceName $namespace)
  (printf "%s.%s.svc.%s" $serviceName $namespace $clusterDomain)
  (printf "%s.%s.svc.%s:*" $serviceName $namespace $clusterDomain)
  "localhost"
  "localhost:*"
  "127.0.0.1"
  "127.0.0.1:*"
  "[::1]"
  "[::1]:*"
  "0.0.0.0"
  "0.0.0.0:*"
  "10.*"
  "192.168.*"
  "172.16.*"
  "172.17.*"
  "172.18.*"
  "172.19.*"
  "172.20.*"
  "172.21.*"
  "172.22.*"
  "172.23.*"
  "172.24.*"
  "172.25.*"
  "172.26.*"
  "172.27.*"
  "172.28.*"
  "172.29.*"
  "172.30.*"
  "172.31.*"
  "fc00:*"
  "fd00:*"
-}}
  {{- if $host -}}
    {{- $hosts = append $hosts $host -}}
  {{- end -}}
{{- end -}}
{{- join "," $hosts -}}
{{- end -}}

{{- define "ai-models.backendCORSAllowedOrigins" -}}
{{- printf "https://%s" (include "ai-models.publicHost" .) -}}
{{- end -}}

{{- define "ai-models.backendReplicas" -}}
{{- if (include "helm_lib_ha_enabled" .) -}}
2
{{- else -}}
1
{{- end -}}
{{- end -}}

{{- define "ai-models.controllerReplicas" -}}
{{- if (include "helm_lib_ha_enabled" .) -}}
2
{{- else -}}
1
{{- end -}}
{{- end -}}

{{- define "ai-models.backendAuthMachineUsername" -}}
{{- $secretName := include "ai-models.backendAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingMachineUsername := (get $existingData "machineUsername" | default "" | b64dec) -}}
{{- coalesce $existingMachineUsername "ai-models-internal" -}}
{{- end -}}

{{- define "ai-models.backendAuthMachinePassword" -}}
{{- $secretName := include "ai-models.backendAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingMachinePassword := (get $existingData "machinePassword" | default "" | b64dec) -}}
{{- $generatedPassword := printf "A1a%s" (randAlphaNum 29) -}}
{{- coalesce $existingMachinePassword $generatedPassword -}}
{{- end -}}

{{- define "ai-models.backendAuthFlaskSecretKey" -}}
{{- $secretName := include "ai-models.backendAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingSecretKey := (get $existingData "flaskServerSecretKey" | default "" | b64dec) -}}
{{- $generatedSecretKey := randAlphaNum 64 -}}
{{- coalesce $existingSecretKey $generatedSecretKey -}}
{{- end -}}

{{- define "ai-models.postgresqlMode" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- default "Managed" (index $postgresql "mode") -}}
{{- end -}}

{{- define "ai-models.postgresqlDatabase" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
{{- $external := (index $postgresql "external") | default dict -}}
{{- default "ai-models" (index $external "database") -}}
{{- else -}}
{{- $managed := (index $postgresql "managed") | default dict -}}
{{- default "ai-models" (index $managed "database") -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlAuthDatabase" -}}
{{- $base := include "ai-models.postgresqlDatabase" . | trim -}}
{{- if gt (len $base) 58 -}}
  {{- $base = (trimSuffix "-" (trunc 58 $base)) -}}
{{- end -}}
{{- printf "%s-auth" $base -}}
{{- end -}}

{{- define "ai-models.postgresqlPort" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
{{- $external := (index $postgresql "external") | default dict -}}
{{- default 5432 (index $external "port") -}}
{{- else -}}
5432
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlUser" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
{{- $external := (index $postgresql "external") | default dict -}}
{{- default "ai-models" (index $external "user") -}}
{{- else -}}
ai-models
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlManagedClassName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- $managed := (index $postgresql "managed") | default dict -}}
{{- default "default" (index $managed "postgresClassName") -}}
{{- end -}}

{{- define "ai-models.postgresqlPasswordSecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
{{- $external := (index $postgresql "external") | default dict -}}
{{- default "" (index $external "existingSecret") -}}
{{- else -}}
{{- include "ai-models.postgresqlSecretName" . -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlPasswordSecretKey" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
{{- $external := (index $postgresql "external") | default dict -}}
{{- default "password" (index $external "existingSecretKey") -}}
{{- else -}}
password
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlManagedPassword" -}}
{{- $secretName := include "ai-models.postgresqlSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingPassword := (get $existingData "password" | default "" | b64dec) -}}
{{- $generatedPassword := printf "A1a%s" (randAlphaNum 37) -}}
{{- coalesce $existingPassword $generatedPassword -}}
{{- end -}}

{{- define "ai-models.backendCryptoKEKPassphrase" -}}
{{- $secretName := include "ai-models.backendCryptoSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingPassphrase := (get $existingData "kekPassphrase" | default "" | b64dec) -}}
{{- $generatedPassphrase := randAlphaNum 64 -}}
{{- coalesce $existingPassphrase $generatedPassphrase -}}
{{- end -}}

{{- define "ai-models.postgresqlSSLMode" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- $raw := "Require" -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
  {{- $external := (index $postgresql "external") | default dict -}}
  {{- $raw = default "Require" (index $external "sslMode") -}}
{{- end -}}
{{- if eq $raw "Disable" -}}disable
{{- else if eq $raw "VerifyCA" -}}verify-ca
{{- else if eq $raw "VerifyFull" -}}verify-full
{{- else -}}require
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlManagedName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
{{- $managed := (index $postgresql "managed") | default dict -}}
{{- default "ai-models" (index $managed "name") -}}
{{- end -}}

{{- define "ai-models.postgresqlManagedType" -}}
{{- if (include "helm_lib_ha_enabled" .) -}}
Cluster
{{- else -}}
Standalone
{{- end -}}
{{- end -}}

{{- define "ai-models.postgresqlManagedTopology" -}}
{{- $className := include "ai-models.postgresqlManagedClassName" . -}}
{{- $postgresClass := lookup "managed-services.deckhouse.io/v1alpha1" "PostgresClass" "" $className -}}
{{- $spec := (get (default (dict) $postgresClass) "spec") | default dict -}}
{{- $topology := (get $spec "topology") | default dict -}}
{{- default "Ignored" (get $topology "defaultTopology") -}}
{{- end -}}

{{- define "ai-models.postgresqlHost" -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
  {{- $moduleValues := (index .Values "aiModels") | default dict -}}
  {{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
  {{- $external := (index $postgresql "external") | default dict -}}
  {{- default "" (index $external "host") -}}
{{- else -}}
{{- printf "d8ms-pg-%s-rw.%s.svc.%s" (include "ai-models.postgresqlManagedName" .) (include "ai-models.namespace" .) (include "ai-models.clusterDomain" .) -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsBucket" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "bucket") -}}
{{- end -}}

{{- define "ai-models.artifactsPathPrefix" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- trimAll "/" (default "ai-models" (index $artifacts "pathPrefix")) -}}
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
{{- include "ai-models.backendTrustCASecretName" . -}}
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

{{- define "ai-models.artifactsRoot" -}}
{{- $bucket := include "ai-models.artifactsBucket" . -}}
{{- $prefix := include "ai-models.artifactsPathPrefix" . -}}
{{- if $prefix -}}
{{- printf "s3://%s/%s" $bucket $prefix -}}
{{- else -}}
{{- printf "s3://%s" $bucket -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrRootDirectory" -}}
/dmcr
{{- end -}}

{{- define "ai-models.dmcrServiceHost" -}}
{{- printf "%s.%s.svc.%s" (include "ai-models.dmcrName" .) (include "ai-models.namespace" .) (include "ai-models.clusterDomain" .) -}}
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
{{- $legacyPassword := (get $authData "password" | default "" | b64dec) -}}
{{- $generatedPassword := printf "A1a%s" (randAlphaNum 37) -}}
{{- coalesce $serverPassword $existingPassword $legacyPassword $generatedPassword -}}
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

{{- define "ai-models.dmcrWriteDockerConfigJSON" -}}
{{- include "ai-models.dmcrDockerConfigJSON" (list . (include "ai-models.dmcrWriteAuthUsername" . | trim) (include "ai-models.dmcrWriteAuthPassword" . | trim)) -}}
{{- end -}}

{{- define "ai-models.dmcrReadDockerConfigJSON" -}}
{{- include "ai-models.dmcrDockerConfigJSON" (list . (include "ai-models.dmcrReadAuthUsername" . | trim) (include "ai-models.dmcrReadAuthPassword" . | trim)) -}}
{{- end -}}

{{- define "ai-models.dmcrWriteHTPasswdEntry" -}}
{{- $secretName := include "ai-models.dmcrAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingEntry := (get $existingData "write.htpasswd" | default "" | b64dec) -}}
{{- $storedPassword := (get $existingData "write.password" | default "" | b64dec) -}}
{{- $desiredPassword := include "ai-models.dmcrWriteAuthPassword" . | trim -}}
{{- if and $existingEntry $storedPassword (eq ($storedPassword | trim) $desiredPassword) -}}
{{- $existingEntry -}}
{{- else -}}
{{- htpasswd (include "ai-models.dmcrWriteAuthUsername" . | trim) $desiredPassword -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrReadHTPasswdEntry" -}}
{{- $secretName := include "ai-models.dmcrAuthSecretName" . -}}
{{- $existingSecret := lookup "v1" "Secret" (include "ai-models.namespace" .) $secretName -}}
{{- $existingData := (get (default (dict) $existingSecret) "data" | default (dict)) -}}
{{- $existingEntry := (get $existingData "read.htpasswd" | default "" | b64dec) -}}
{{- $storedPassword := (get $existingData "read.password" | default "" | b64dec) -}}
{{- $desiredPassword := include "ai-models.dmcrReadAuthPassword" . | trim -}}
{{- if and $existingEntry $storedPassword (eq ($storedPassword | trim) $desiredPassword) -}}
{{- $existingEntry -}}
{{- else -}}
{{- htpasswd (include "ai-models.dmcrReadAuthUsername" . | trim) $desiredPassword -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.dmcrHTPasswd" -}}
{{- printf "%s\n%s" (include "ai-models.dmcrWriteHTPasswdEntry" . | trim) (include "ai-models.dmcrReadHTPasswdEntry" . | trim) -}}
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
  {{- if and (eq (include "ai-models.postgresqlMode" .) "Managed") (not (has "managed-postgres" $enabledModules)) -}}
    {{- fail "ai-models.postgresql.mode=Managed requires the managed-postgres module" -}}
  {{- end -}}
  {{- if and (eq (include "ai-models.postgresqlMode" .) "External") (not (include "ai-models.postgresqlHost" . | trim)) -}}
    {{- fail "ai-models.postgresql.external.host is required when postgresql.mode=External" -}}
  {{- end -}}
  {{- if and (eq (include "ai-models.postgresqlMode" .) "External") (not (include "ai-models.postgresqlPasswordSecretName" . | trim)) -}}
    {{- fail "ai-models.postgresql.external.existingSecret is required when postgresql.mode=External" -}}
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
  {{- $publicationWorkVolumeType := include "ai-models.publicationWorkVolumeType" . | trim -}}
  {{- if not (or (eq $publicationWorkVolumeType "EmptyDir") (eq $publicationWorkVolumeType "PersistentVolumeClaim")) -}}
    {{- fail "ai-models.internal.publicationRuntime.workVolume.type must be either EmptyDir or PersistentVolumeClaim" -}}
  {{- end -}}
  {{- if and (eq $publicationWorkVolumeType "PersistentVolumeClaim") (gt (include "ai-models.publicationMaxConcurrentWorkers" . | int) 1) -}}
    {{- fail "ai-models.internal.publicationRuntime.maxConcurrentWorkers must be 1 when workVolume.type=PersistentVolumeClaim" -}}
  {{- end -}}
  {{- if not (has "user-authn" $enabledModules) -}}
    {{- fail "ai-models requires the user-authn module for browser SSO" -}}
  {{- end -}}
  {{- if not (include "ai-models.hasAPI" (list . "deckhouse.io/v1/DexClient")) -}}
    {{- fail "ai-models requires the deckhouse.io/v1 DexClient API" -}}
  {{- end -}}
  {{- if not (include "ai-models.authSSODexClientAllowedGroupsYAML" . | trim) -}}
    {{- fail "ai-models.auth.sso.allowedGroups/adminGroups must contain at least one non-empty group" -}}
  {{- end -}}
{{- end -}}
{{- end -}}
