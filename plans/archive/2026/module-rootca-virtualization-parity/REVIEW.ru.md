# Review Gate

## Findings

- Критичных замечаний по slice нет.

## Scope check

- Изменение осталось в internal runtime TLS boundary: public API, RBAC,
  ingress TLS и DMCR auth/storage не менялись этим slice.
- Новый `rootCA` surface находится в `aiModels.internal` и не становится
  user-facing contract.
- Паттерн соответствует virtualization: discovery hook восстанавливает
  persisted module CA, TLS hooks используют `CommonCAValuesPath`, templates
  рендерят Secret и ConfigMap.

## Residual risks

- Первый rollout после включения common CA может перезаписать endpoint TLS
  Secrets, если текущие controller/DMCR certs были подписаны разными CA. Это
  ожидаемая миграция к единому internal CA и должна привести к rollout
  затронутых pods через существующие checksum annotations.

## Evidence

- `(cd images/hooks && go test ./...)` — OK.
- `make helm-template` — OK.
- `make kubeconform` — OK.
- `python3 tools/helm-tests/validate-renders.py tools/kubeconform/renders` — OK.
- `git diff --check` — OK.
- `make verify` — OK.
