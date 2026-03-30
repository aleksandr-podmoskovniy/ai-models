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

{{- define "ai-models.configMapName" -}}
ai-models-runtime
{{- end -}}

{{- define "ai-models.postgresqlSecretName" -}}
ai-models-postgresql
{{- end -}}

{{- define "ai-models.artifactsManagedSecretName" -}}
ai-models-artifacts
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

{{- define "ai-models.backendPort" -}}
5000
{{- end -}}

{{- define "ai-models.backendMetricsDir" -}}
/tmp/ai-models-metrics
{{- end -}}

{{- define "ai-models.backendReplicas" -}}
{{- if (include "helm_lib_ha_enabled" .) -}}
2
{{- else -}}
1
{{- end -}}
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
{{- default "ai_models" (index $external "database") -}}
{{- else -}}
{{- $managed := (index $postgresql "managed") | default dict -}}
{{- default "ai_models" (index $managed "database") -}}
{{- end -}}
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
{{- default "ai_models" (index $external "user") -}}
{{- else -}}
ai_models
{{- end -}}
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

{{- define "ai-models.postgresqlHost" -}}
{{- if eq (include "ai-models.postgresqlMode" .) "External" -}}
  {{- $moduleValues := (index .Values "aiModels") | default dict -}}
  {{- $postgresql := (index $moduleValues "postgresql") | default dict -}}
  {{- $external := (index $postgresql "external") | default dict -}}
  {{- default "" (index $external "host") -}}
{{- else -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $clusterConfiguration := (index $globalValues "clusterConfiguration") | default dict -}}
{{- $discovery := (index $globalValues "discovery") | default dict -}}
{{- $clusterDomain := default (index $discovery "clusterDomain") (index $clusterConfiguration "clusterDomain") -}}
{{- printf "d8ms-pg-%s-rw.%s.svc.%s" (include "ai-models.postgresqlManagedName" .) (include "ai-models.namespace" .) $clusterDomain -}}
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

{{- define "ai-models.dexEnabled" -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $enabledModules := (index $globalValues "enabledModules") | default list -}}
{{- $modulesValues := (index $globalValues "modules") | default dict -}}
{{- $publicDomainTemplate := default "" (index $modulesValues "publicDomainTemplate") -}}
{{- if and $publicDomainTemplate (has "user-authn" $enabledModules) (ne (include "helm_lib_module_https_mode" .) "Disabled") -}}
true
{{- end -}}
{{- end -}}

{{- define "ai-models.dexAuthServiceHost" -}}
{{- $globalValues := (index .Values "global") | default dict -}}
{{- $clusterConfiguration := (index $globalValues "clusterConfiguration") | default dict -}}
{{- $discovery := (index $globalValues "discovery") | default dict -}}
{{- $clusterDomain := default (index $discovery "clusterDomain") (index $clusterConfiguration "clusterDomain") -}}
{{- printf "https://ai-models-dex-authenticator.%s.svc.%s/dex-authenticator/auth" (include "ai-models.namespace" .) $clusterDomain -}}
{{- end -}}

{{- define "ai-models.dexAuthSignInURL" -}}
{{- printf "https://$host/dex-authenticator/sign_in" -}}
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
  {{- if not (has "user-authn" $enabledModules) -}}
    {{- fail "ai-models requires the user-authn module for Dex SSO" -}}
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
{{- end -}}
{{- end -}}
