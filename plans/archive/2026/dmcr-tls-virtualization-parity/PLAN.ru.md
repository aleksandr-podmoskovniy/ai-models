# План: DMCR TLS parity с virtualization

## Current phase

Этап 1: publication/runtime baseline. DMCR является внутренним publication
backend, поэтому TLS wiring должен быть deterministic module state, как DVCR в
`virtualization`.

## Orchestration

Режим `solo` с ограничением: AGENTS требует delegation для runtime/TLS задач,
но текущий turn не разрешает subagents явно. Сравнение с `virtualization`
выполняется локально.

## Slices

1. Зафиксировать virtualization DVCR baseline.
   - Файлы: `NOTES.ru.md`.
   - Проверка: documented decision.

2. Добавить DMCR TLS hook и schema.
   - Файлы: `images/hooks/pkg/hooks/tls_certificates_dmcr`,
     `images/hooks/cmd/ai-models-hooks/register.go`,
     `images/hooks/pkg/settings/certificate.go`, `openapi/values.yaml`,
     `fixtures/module-values.yaml`.
   - Проверка: `(cd images/hooks && go test ./...)`.

3. Перевести DMCR TLS Secret и checksum на values-backed cert.
   - Файлы: `templates/dmcr/secret.yaml`, `templates/_helpers.tpl`.
   - Проверки: `make helm-template`, `make kubeconform`.

4. Добавить render guardrail.
   - Файлы: `tools/helm-tests/validate-renders.py`.
   - Проверка:
     `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`.

## Rollback point

Вернуть `templates/dmcr/secret.yaml` и `dmcrTLSSecretRestartChecksum` на
предыдущий `lookup`/`genCA` path, удалить DMCR TLS hook и schema values.

## Final validation

- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Result

- DMCR TLS переведён на hook-owned `aiModels.internal.dmcr.cert`.
- `templates/dmcr/secret.yaml` больше не использует `lookup`, `genCA`,
  `genSignedCert` для TLS и не рендерит приватный `ca.key`.
- DMCR TLS Secret теперь `kubernetes.io/tls`; публичный client CA Secret
  содержит только `ca.crt`.
- `dmcrTLSSecretRestartChecksum` считается из values-backed `ca/crt/key`, а не
  из existing Secret annotation.
- Render validation запрещает возврат Helm-time TLS generation для controller
  webhook и DMCR TLS.
- Проверки пройдены: `(cd images/hooks && go test ./...)`, `make helm-template`,
  `make kubeconform`,
  `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`,
  `git diff --check`, `make verify`.
