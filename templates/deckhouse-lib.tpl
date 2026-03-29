{{- /*
Repo-local subset of deckhouse_lib_helm helpers.
Baseline synced against deckhouse/helm_lib/charts/deckhouse_lib_helm version 1.70.1.
Keep custom nil-safe defaults unless a deliberate DKP baseline refresh says otherwise.
*/ -}}
{{- define "helm_lib_module_camelcase_name" -}}
{{- $moduleName := "" -}}
{{- if (kindIs "string" .) -}}
{{- $moduleName = . | trimAll "\"" -}}
{{- else -}}
{{- $moduleName = .Chart.Name -}}
{{- end -}}
{{ $moduleName | replace "-" "_" | camelcase | untitle }}
{{- end -}}

{{- define "helm_lib_module_kebabcase_name" -}}
{{- $moduleName := "" -}}
{{- if (kindIs "string" .) -}}
{{- $moduleName = . | trimAll "\"" -}}
{{- else -}}
{{- $moduleName = .Chart.Name -}}
{{- end -}}
{{ $moduleName | kebabcase }}
{{- end -}}

{{- define "helm_lib_module_labels" -}}
{{- $context := index . 0 -}}
labels:
  heritage: deckhouse
  module: {{ $context.Chart.Name }}
  {{- if eq (len .) 2 }}
    {{- $extra := index . 1 }}
    {{- range $key, $value := $extra }}
  {{ $key }}: {{ $value | quote }}
    {{- end }}
  {{- end }}
{{- end -}}

{{- define "helm_lib_module_public_domain" -}}
{{- $context := index . 0 -}}
{{- $name_portion := index . 1 -}}
{{- if not (contains "%s" $context.Values.global.modules.publicDomainTemplate) }}
{{ fail "Error!!! global.modules.publicDomainTemplate must contain \"%s\" pattern to render service fqdn!" }}
{{- end }}
{{- printf $context.Values.global.modules.publicDomainTemplate $name_portion }}
{{- end -}}

{{- define "helm_lib_module_ingress_class" -}}
{{- $context := . -}}
{{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) | default dict -}}
{{- if hasKey $moduleValues "ingressClass" -}}
{{- $moduleValues.ingressClass -}}
{{- else if hasKey $context.Values.global.modules "ingressClass" -}}
{{- $context.Values.global.modules.ingressClass -}}
{{- end -}}
{{- end -}}

{{- define "helm_lib_https_values" -}}
{{- $context := . -}}
{{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) | default dict -}}
{{- $mode := "" -}}
{{- $certManagerClusterIssuerName := "" -}}
{{- if hasKey $moduleValues "https" -}}
  {{- if hasKey $moduleValues.https "mode" -}}
    {{- $mode = $moduleValues.https.mode -}}
    {{- if eq $mode "CertManager" -}}
      {{- if not (hasKey $moduleValues.https "certManager") -}}
        {{- cat "<module>.https.certManager.clusterIssuerName is mandatory when <module>.https.mode is set to CertManager" | fail -}}
      {{- end -}}
      {{- if hasKey $moduleValues.https.certManager "clusterIssuerName" -}}
        {{- $certManagerClusterIssuerName = $moduleValues.https.certManager.clusterIssuerName -}}
      {{- else -}}
        {{- cat "<module>.https.certManager.clusterIssuerName is mandatory when <module>.https.mode is set to CertManager" | fail -}}
      {{- end -}}
    {{- end -}}
  {{- else -}}
    {{- cat "<module>.https.mode is mandatory when <module>.https is defined" | fail -}}
  {{- end -}}
{{- end -}}
{{- if empty $mode -}}
  {{- $mode = $context.Values.global.modules.https.mode -}}
  {{- if eq $mode "CertManager" -}}
    {{- $certManagerClusterIssuerName = $context.Values.global.modules.https.certManager.clusterIssuerName -}}
  {{- end -}}
{{- end -}}
{{- if not (has $mode (list "Disabled" "CertManager" "CustomCertificate" "OnlyInURI")) -}}
  {{- cat "Unknown https.mode:" $mode | fail -}}
{{- end -}}
{{- if and (eq $mode "CertManager") (not ($context.Values.global.enabledModules | has "cert-manager")) -}}
  {{- cat "https.mode has value CertManager but cert-manager module not enabled" | fail -}}
{{- end -}}
mode: {{ $mode }}
{{- if eq $mode "CertManager" }}
certManager:
  clusterIssuerName: {{ $certManagerClusterIssuerName }}
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_https_mode" -}}
{{- $httpsValues := include "helm_lib_https_values" . | fromYaml -}}
{{- $httpsValues.mode -}}
{{- end -}}

{{- define "helm_lib_module_https_cert_manager_cluster_issuer_name" -}}
{{- $httpsValues := include "helm_lib_https_values" . | fromYaml -}}
{{- $httpsValues.certManager.clusterIssuerName -}}
{{- end -}}

{{- define "helm_lib_module_https_ingress_tls_enabled" -}}
{{- $mode := include "helm_lib_module_https_mode" . -}}
{{- if or (eq "CertManager" $mode) (eq "CustomCertificate" $mode) -}}
not empty string
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_https_secret_name" -}}
{{- $context := index . 0 -}}
{{- $secretNamePrefix := index . 1 -}}
{{- $mode := include "helm_lib_module_https_mode" $context -}}
{{- if eq $mode "CertManager" -}}
{{- $secretNamePrefix -}}
{{- else if eq $mode "CustomCertificate" -}}
{{- printf "%s-customcertificate" $secretNamePrefix -}}
{{- else -}}
{{- fail "https.mode must be CustomCertificate or CertManager" -}}
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_https_copy_custom_certificate" -}}
{{- $context := index . 0 -}}
{{- $namespace := index . 1 -}}
{{- $secretNamePrefix := index . 2 -}}
{{- $mode := include "helm_lib_module_https_mode" $context -}}
{{- if eq $mode "CustomCertificate" -}}
  {{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) | default dict -}}
  {{- $internalValues := (index $moduleValues "internal") | default dict -}}
  {{- $customCertificateData := (index $internalValues "customCertificateData") -}}
  {{- if not $customCertificateData -}}
    {{- fail (printf "internal.customCertificateData is required to copy custom certificate for secret prefix '%s'" $secretNamePrefix) -}}
  {{- end -}}
  {{- $tlsCrt := index $customCertificateData "tls.crt" -}}
  {{- $tlsKey := index $customCertificateData "tls.key" -}}
  {{- $secretName := include "helm_lib_module_https_secret_name" (list $context $secretNamePrefix) -}}
---
apiVersion: v1
kind: Secret
metadata:
  name: {{ $secretName }}
  namespace: {{ $namespace }}
  {{- include "helm_lib_module_labels" (list $context) | nindent 2 }}
type: kubernetes.io/tls
data:
  {{- if (hasKey $customCertificateData "ca.crt") }}
  ca.crt: {{ index $customCertificateData "ca.crt" | b64enc }}
  {{- end }}
  tls.crt: {{ $tlsCrt | b64enc }}
  tls.key: {{ $tlsKey | b64enc }}
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_image" -}}
{{- $context := index . 0 -}}
{{- $containerName := index . 1 | trimAll "\"" -}}
{{- $rawModuleName := $context.Chart.Name -}}
{{- if ge (len .) 3 }}
  {{- $rawModuleName = (index . 2) -}}
{{- end }}
{{- $moduleName := (include "helm_lib_module_camelcase_name" $rawModuleName) -}}
{{- $imageDigest := index $context.Values.global.modulesImages.digests $moduleName $containerName -}}
{{- if not $imageDigest }}
  {{- fail (printf "Image %s.%s has no digest" $moduleName $containerName) }}
{{- end }}
{{- $registryBase := $context.Values.global.modulesImages.registry.base -}}
{{- if index $context.Values $moduleName }}
  {{- if index $context.Values $moduleName "registry" }}
    {{- if index $context.Values $moduleName "registry" "base" }}
      {{- $host := trimAll "/" (index $context.Values $moduleName "registry" "base") }}
      {{- $path := trimAll "/" (include "helm_lib_module_kebabcase_name" $rawModuleName) }}
      {{- $registryBase = join "/" (list $host $path) }}
    {{- end }}
  {{- end }}
{{- end }}
{{- printf "%s@%s" $registryBase $imageDigest }}
{{- end -}}

{{- define "helm_lib_is_ha_to_value" -}}
{{- $context := index . 0 -}}
{{- $yes := index . 1 -}}
{{- $no := index . 2 -}}
{{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) | default dict -}}
{{- if hasKey $moduleValues "highAvailability" -}}
  {{- if $moduleValues.highAvailability -}}{{- $yes -}}{{- else -}}{{- $no -}}{{- end -}}
{{- else if hasKey $context.Values.global "highAvailability" -}}
  {{- if $context.Values.global.highAvailability -}}{{- $yes -}}{{- else -}}{{- $no -}}{{- end -}}
{{- else -}}
  {{- if $context.Values.global.discovery.clusterControlPlaneIsHighlyAvailable -}}{{- $yes -}}{{- else -}}{{- $no -}}{{- end -}}
{{- end -}}
{{- end -}}

{{- define "helm_lib_ha_enabled" -}}
{{- $context := . -}}
{{- $moduleValues := (index $context.Values (include "helm_lib_module_camelcase_name" $context)) | default dict -}}
{{- if hasKey $moduleValues "highAvailability" -}}
  {{- if $moduleValues.highAvailability -}}"not empty string"{{- end -}}
{{- else if hasKey $context.Values.global "highAvailability" -}}
  {{- if $context.Values.global.highAvailability -}}"not empty string"{{- end -}}
{{- else -}}
  {{- if $context.Values.global.discovery.clusterControlPlaneIsHighlyAvailable -}}"not empty string"{{- end -}}
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_generate_common_name" -}}
{{- $context := index . 0 -}}
{{- $namePortion := index . 1 -}}
{{- $domain := include "helm_lib_module_public_domain" (list $context $namePortion) -}}
{{- if le (len $domain) 64 -}}
commonName: {{ $domain }}
{{- end -}}
{{- end -}}

{{- define "helm_lib_module_pod_security_context_run_as_user_deckhouse_with_writable_fs" -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 64535
  runAsGroup: 64535
  fsGroup: 64535
{{- end -}}
