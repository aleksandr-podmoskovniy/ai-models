{{- define "ai-models.moduleName" -}}
ai-models
{{- end -}}

{{- define "ai-models.fullname" -}}
ai-models
{{- end -}}

{{- define "ai-models.namespace" -}}
d8-ai-models
{{- end -}}

{{- define "ai-models.serviceName" -}}
ai-models
{{- end -}}

{{- define "ai-models.serviceAccountName" -}}
ai-models
{{- end -}}

{{- define "ai-models.controllerName" -}}
ai-models-controller
{{- end -}}

{{- define "ai-models.controllerServiceAccountName" -}}
{{- include "ai-models.controllerName" . -}}
{{- end -}}

{{- define "ai-models.controllerMetricsServiceName" -}}
{{- printf "%s-metrics" (include "ai-models.controllerName" .) -}}
{{- end -}}

{{- define "ai-models.configMapName" -}}
ai-models-runtime
{{- end -}}

{{- define "ai-models.postgresqlSecretName" -}}
ai-models-postgresql
{{- end -}}

{{- define "ai-models.artifactsManagedSecretName" -}}
ai-models-artifacts
{{- end -}}

{{- define "ai-models.publicationRegistryManagedSecretName" -}}
ai-models-publication-registry
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

{{- define "ai-models.artifactsRegion" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "us-east-1" (index $artifacts "region") -}}
{{- end -}}

{{- define "ai-models.artifactsInlineAccessKey" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "accessKey") -}}
{{- end -}}

{{- define "ai-models.artifactsInlineSecretKey" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $artifacts := (index $moduleValues "artifacts") | default dict -}}
{{- default "" (index $artifacts "secretKey") -}}
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

{{- define "ai-models.artifactsHasInlineCredentials" -}}
{{- $accessKey := include "ai-models.artifactsInlineAccessKey" . | trim -}}
{{- $secretKey := include "ai-models.artifactsInlineSecretKey" . | trim -}}
{{- if and $accessKey $secretKey -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsHasSecretReference" -}}
{{- if (include "ai-models.artifactsCredentialsSecretName" . | trim) -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsMountedCASecretName" -}}
{{- $explicit := include "ai-models.artifactsCASecretName" . | trim -}}
{{- if $explicit -}}
{{- $explicit -}}
{{- else -}}
  {{- $credentialsSecretName := include "ai-models.artifactsCredentialsSecretName" . | trim -}}
  {{- if $credentialsSecretName -}}
{{- $credentialsSecretName -}}
  {{- else if (include "ai-models.hasPlatformTrustCA" . | trim) -}}
{{- include "ai-models.backendTrustCASecretName" . -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.artifactsResolvedSecretName" -}}
{{- $external := include "ai-models.artifactsCredentialsSecretName" . | trim -}}
{{- if $external -}}
{{- $external -}}
{{- else -}}
{{- include "ai-models.artifactsManagedSecretName" . -}}
{{- end -}}
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

{{- define "ai-models.publicationRegistryRepositoryPrefix" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- trimAll "/" (default "" (index $publicationRegistry "repositoryPrefix")) -}}
{{- end -}}

{{- define "ai-models.publicationRegistryInlineUsername" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- default "" (index $publicationRegistry "username") -}}
{{- end -}}

{{- define "ai-models.publicationRegistryInlinePassword" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- default "" (index $publicationRegistry "password") -}}
{{- end -}}

{{- define "ai-models.publicationRegistryCredentialsSecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- default "" (index $publicationRegistry "credentialsSecretName") -}}
{{- end -}}

{{- define "ai-models.publicationRegistryCASecretName" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- default "" (index $publicationRegistry "caSecretName") -}}
{{- end -}}

{{- define "ai-models.publicationRegistryHasInlineCredentials" -}}
{{- $username := include "ai-models.publicationRegistryInlineUsername" . | trim -}}
{{- $password := include "ai-models.publicationRegistryInlinePassword" . | trim -}}
{{- if and $username $password -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.publicationRegistryHasSecretReference" -}}
{{- if (include "ai-models.publicationRegistryCredentialsSecretName" . | trim) -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.publicationRegistryMountedCASecretName" -}}
{{- $explicit := include "ai-models.publicationRegistryCASecretName" . | trim -}}
{{- if $explicit -}}
{{- $explicit -}}
{{- else -}}
  {{- if (include "ai-models.hasPlatformTrustCA" . | trim) -}}
{{- include "ai-models.backendTrustCASecretName" . -}}
  {{- end -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.publicationRegistryResolvedSecretName" -}}
{{- $external := include "ai-models.publicationRegistryCredentialsSecretName" . | trim -}}
{{- if $external -}}
{{- $external -}}
{{- else -}}
{{- include "ai-models.publicationRegistryManagedSecretName" . -}}
{{- end -}}
{{- end -}}

{{- define "ai-models.publicationRegistryInsecure" -}}
{{- $moduleValues := (index .Values "aiModels") | default dict -}}
{{- $publicationRegistry := (index $moduleValues "publicationRegistry") | default dict -}}
{{- if and (hasKey $publicationRegistry "insecure") (index $publicationRegistry "insecure") -}}
true
{{- else -}}
false
{{- end -}}
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
  {{- if and (include "ai-models.artifactsInlineAccessKey" . | trim) (not (include "ai-models.artifactsInlineSecretKey" . | trim)) -}}
    {{- fail "ai-models.artifacts.secretKey is required when ai-models.artifacts.accessKey is set" -}}
  {{- end -}}
  {{- if and (include "ai-models.artifactsInlineSecretKey" . | trim) (not (include "ai-models.artifactsInlineAccessKey" . | trim)) -}}
    {{- fail "ai-models.artifacts.accessKey is required when ai-models.artifacts.secretKey is set" -}}
  {{- end -}}
  {{- if and (include "ai-models.artifactsHasInlineCredentials" . | trim) (include "ai-models.artifactsHasSecretReference" . | trim) -}}
    {{- fail "ai-models.artifacts.credentialsSecretName cannot be used together with inline accessKey/secretKey" -}}
  {{- end -}}
  {{- if not (or (include "ai-models.artifactsHasInlineCredentials" . | trim) (include "ai-models.artifactsHasSecretReference" . | trim)) -}}
    {{- fail "ai-models.artifacts requires either credentialsSecretName or inline accessKey and secretKey" -}}
  {{- end -}}
  {{- if not (include "ai-models.publicationRegistryRepositoryPrefix" . | trim) -}}
    {{- fail "ai-models.publicationRegistry.repositoryPrefix is required" -}}
  {{- end -}}
  {{- if and (include "ai-models.publicationRegistryInlineUsername" . | trim) (not (include "ai-models.publicationRegistryInlinePassword" . | trim)) -}}
    {{- fail "ai-models.publicationRegistry.password is required when ai-models.publicationRegistry.username is set" -}}
  {{- end -}}
  {{- if and (include "ai-models.publicationRegistryInlinePassword" . | trim) (not (include "ai-models.publicationRegistryInlineUsername" . | trim)) -}}
    {{- fail "ai-models.publicationRegistry.username is required when ai-models.publicationRegistry.password is set" -}}
  {{- end -}}
  {{- if and (include "ai-models.publicationRegistryHasInlineCredentials" . | trim) (include "ai-models.publicationRegistryHasSecretReference" . | trim) -}}
    {{- fail "ai-models.publicationRegistry.credentialsSecretName cannot be used together with inline username/password" -}}
  {{- end -}}
  {{- if not (or (include "ai-models.publicationRegistryHasInlineCredentials" . | trim) (include "ai-models.publicationRegistryHasSecretReference" . | trim)) -}}
    {{- fail "ai-models.publicationRegistry requires either credentialsSecretName or inline username and password" -}}
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
