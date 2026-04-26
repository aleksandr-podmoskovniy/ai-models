# Review Gate

## Findings

Критичных замечаний по controller webhook TLS slice нет.

## Проверки

- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Остаточные риски

- Первый live upgrade должен полагаться на OnBeforeHelm hook order: hook
  должен успеть заполнить `aiModels.internal.controller.cert` до render.
- Module common rootCA не вводился намеренно; это отдельный slice, если будет
  нужен общий CA для всех internal endpoints.
