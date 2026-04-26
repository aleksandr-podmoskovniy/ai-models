# Review Gate

## Findings

Критичных замечаний по DMCR TLS slice нет.

## Проверки

- `(cd images/hooks && go test ./...)`
- `make helm-template`
- `make kubeconform`
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders`
- `git diff --check`
- `make verify`

## Остаточные риски

- Первый live upgrade может один раз перезапустить DMCR: TLS Secret type/data и
  restart checksum теперь считаются из hook-owned values, а не из old
  annotation/lookup state.
- DMCR auth password/salt generation всё ещё использует Helm `lookup`/`rand`.
  Это не TLS drift и осталось следующим кандидатом на перенос в hook/internal
  values.
