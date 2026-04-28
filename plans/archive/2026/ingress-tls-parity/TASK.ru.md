# Ingress TLS parity

## 1. Заголовок

Выровнять upload-gateway Ingress TLS с Deckhouse/virtualization паттерном.

## 2. Контекст

Live e2e был остановлен после замечания, что сертификат в Ingress прокинут неверно. Текущий шаблон `templates/upload-gateway/ingress.yaml` задаёт `spec.tls.secretName`, но не создаёт `cert-manager.io/Certificate` для режима `CertManager`. В virtualization это сделано отдельным `templates/certificate.yaml`: Ingress ссылается на `helm_lib_module_https_secret_name`, а Certificate владеет тем же Secret.

## 3. Scope

- Добавить Certificate template для upload public host в режиме `CertManager`.
- Использовать тот же secret name helper, что и Ingress.
- Сохранить CustomCertificate copy через helm-lib.
- Добавить render validation, чтобы CertManager render не имел Ingress TLS без Certificate.

## 4. Non-goals

- Не менять public host, ingress class, upload API или auth.
- Не менять root CA/internal TLS для controller/DMCR.
- Не продолжать destructive e2e до отдельной команды.

## 5. Acceptance Criteria

- В CertManager render есть `Certificate` в `d8-ai-models`, который пишет `secretName: ingress-tls`.
- Certificate `dnsNames` совпадает с upload-gateway Ingress host.
- В CustomCertificate render Ingress использует `ingress-tls-customcertificate`, а Secret с этим именем рендерится как `kubernetes.io/tls`.
- `make helm-template`, `make kubeconform`, `git diff --check` проходят.

## 6. RBAC/Exposure

Публичный endpoint и RBAC не меняются. Меняется только TLS Secret ownership for existing upload-gateway Ingress.
