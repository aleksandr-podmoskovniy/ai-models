# План: DMCR auth parity с virtualization

## Current phase

Этап 1: publication/runtime baseline. DMCR является internal publication
backend, его auth state должен быть explicit module internal state, а не
stateful Helm render.

## Orchestration

Режим `solo` с ограничением: AGENTS требует delegation для auth/runtime задач,
но текущий turn не разрешает subagents явно. Сравнение с `virtualization`
выполняется локально.

## Slices

1. Зафиксировать DVCR baseline.
   - Файлы: `NOTES.ru.md`.
   - Проверка: documented decision.

2. Добавить DMCR auth hook.
   - Файлы: `images/hooks/pkg/hooks/generate_dmcr_auth`,
     `images/hooks/cmd/ai-models-hooks/register.go`.
   - Проверка: `(cd images/hooks && go test ./pkg/hooks/generate_dmcr_auth)`.

3. Перевести values/schema/templates.
   - Файлы: `openapi/values.yaml`, `fixtures/module-values.yaml`,
     `templates/dmcr/secret.yaml`, `templates/_helpers.tpl`.
   - Проверки: `make helm-template`, `make kubeconform`.

4. Добавить regression guardrail.
   - Файлы: `tools/helm-tests/validate-renders.py`.
   - Проверка:
     `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`.

## Rollback point

Вернуть DMCR auth helpers в `templates/_helpers.tpl`, удалить hook import,
schema/fixture auth values и template projection из internal auth.

## Final validation

- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Result

- DMCR auth state перенесён в hook-owned
  `aiModels.internal.dmcr.auth.{writePassword,readPassword,writeHtpasswd,readHtpasswd,salt}`.
- Hook мигрирует existing `ai-models-dmcr-auth`, `ai-models-dmcr-auth-write`
  и `ai-models-dmcr-auth-read`; если state пустой или htpasswd повреждён,
  генерирует корректные значения.
- `templates/dmcr/secret.yaml` теперь только проецирует auth values.
- Удалены Helm helpers с `lookup`, `randAlphaNum` и Helm `htpasswd` для DMCR
  auth.
- `dmcrAuthSecretRestartChecksum` считается из explicit internal auth values.
- Render validation запрещает возвращение DMCR auth stateful render helpers.
- Проверки пройдены: `(cd images/hooks && go test ./...)`, `make helm-template`,
  `make kubeconform`,
  `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`,
  `git diff --check`, `make verify`.
