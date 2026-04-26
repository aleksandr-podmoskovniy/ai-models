# План: TLS parity с virtualization

## Current phase

Этап 1: publication/runtime baseline. TLS webhook wiring относится к module
runtime shell и должен быть deterministic, как в `virtualization`.

## Orchestration

Режим `solo` с ограничением: AGENTS требует delegation для TLS/runtime задач,
но текущий turn не разрешает subagents явно. Поэтому сравнение с
`virtualization` выполняется локально, а финальная проверка закрывается
`review-gate`.

## Slices

1. Зафиксировать virtualization baseline.
   - Файлы: `NOTES.ru.md`.
   - Проверка: documented decision.
   - Результат: понятная причина удаления Helm-time cert generation.

2. Добавить hook и values schema.
   - Файлы: `images/hooks/pkg/hooks/tls_certificates_controller`,
     `images/hooks/cmd/ai-models-hooks/register.go`,
     `images/hooks/pkg/settings`, `openapi/values.yaml`.
   - Проверки: `go test ./...` in `images/hooks`.
   - Результат: internal controller cert values appear as owned module state.

3. Перевести templates на values-backed cert.
   - Файлы: `templates/controller/webhook.yaml`,
     `templates/controller/deployment.yaml`, fixtures if needed.
   - Проверки: `make helm-template`, `make kubeconform`.
   - Результат: render no longer generates random TLS data.

4. Добавить render guardrail.
   - Файлы: `tools/helm-tests/validate-renders.py`.
   - Проверка: `python3 tools/helm-tests/validate-renders.py`.
   - Результат: regression guard against `genCA`/`lookup` in controller webhook.

## Rollback point

До удаления template generation можно вернуть `webhook.yaml` на прежний
`lookup`/`genCA` path. После hook registration rollback требует удалить hook
import, values schema и вернуть старую Secret генерацию.

## Final validation

- `go test ./...` в `images/hooks`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `make verify`
- `git diff --check`

## Result

- Controller webhook TLS переведён на hook-owned
  `aiModels.internal.controller.cert`.
- `templates/controller/webhook.yaml` больше не использует `lookup`,
  `genCA`, `genSignedCert` и не рендерит `ca.key`.
- Render validation теперь запрещает возвращение Helm-time TLS generation для
  controller webhook.
- Проверки пройдены:
  - `(cd images/hooks && go test ./...)`
  - `make helm-template`
  - `make kubeconform`
  - `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
  - `git diff --check`
  - `make verify`
