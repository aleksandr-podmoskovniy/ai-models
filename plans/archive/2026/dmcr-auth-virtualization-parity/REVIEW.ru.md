# Review Gate

## Findings

Критичных замечаний по DMCR auth slice нет.

## Проверки

- `(cd images/hooks && go test ./pkg/hooks/generate_dmcr_auth)`
- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Остаточные риски

- Первый live upgrade должен выполнить OnBeforeHelm hook до render, чтобы
  `aiModels.internal.dmcr.auth` был заполнен. Hook мигрирует existing Secrets,
  поэтому штатный upgrade не должен менять пароли.
- Если в кластере одновременно отсутствуют old DMCR auth Secrets и internal
  auth values, hook сгенерирует новые credentials. Это ожидаемый bootstrap path.
