# План: internal rootCA parity с virtualization

## Current phase

Этап 1: publication/runtime baseline. Изменение касается internal runtime TLS
wiring и не меняет public API.

## Orchestration

Режим `solo`: user не разрешал subagents в текущем turn; baseline берётся из
локального `virtualization`.

## Slices

1. Зафиксировать virtualization rootCA baseline. — выполнено
   - Файлы: `NOTES.ru.md`.

2. Добавить hook/schema/templates. — выполнено
   - Файлы: `images/hooks/pkg/hooks/root_ca_discovery`,
     `images/hooks/cmd/ai-models-hooks/register.go`, `openapi/values.yaml`,
     `fixtures/module-values.yaml`, `templates/rootca-*.yaml`.
   - Проверки: `(cd images/hooks && go test ./...)`, `make helm-template`.

3. Подключить CommonCAValuesPath. — выполнено
   - Файлы: `images/hooks/pkg/hooks/tls_certificates_*`.
   - Проверки: `(cd images/hooks && go test ./...)`.

4. Добавить guardrail. — выполнено
   - Файлы: `tools/helm-tests/validate-renders.py`.
   - Проверка:
     `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`.

## Rollback point

Удалить rootCA hook/templates/schema, убрать `CommonCAValuesPath` из TLS hooks.

## Final validation

- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Result

- Добавлен hook `root_ca_discovery`, который восстанавливает persisted
  `Secret/ai-models-ca` в `aiModels.internal.rootCA` до Helm render.
- Controller webhook TLS и DMCR TLS теперь используют общий
  `CommonCAValuesPath: aiModels.internal.rootCA`.
- Root CA рендерится как internal `Secret/ai-models-ca` и
  `ConfigMap/ai-models-ca`; endpoint cert Secret остаются values-backed и не
  возвращают Helm-time `lookup/genCA/genSignedCert`.
- Render validator теперь проверяет общий CA path, rootCA templates и
  values-backed TLS/auth guardrails.

## Validation result

- `(cd images/hooks && go test ./pkg/hooks/root_ca_discovery ./pkg/hooks/tls_certificates_controller ./pkg/hooks/tls_certificates_dmcr)` — OK.
- `(cd images/hooks && go test ./...)` — OK.
- `make helm-template` — OK.
- `make kubeconform` — OK.
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders` — OK.
- `git diff --check` — OK.
- `make verify` — OK.
