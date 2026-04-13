# REVIEW

## Slice

Slice 1. Harden kubeconform bootstrap download.

## Что проверено

- фикc не меняет semantics самой `kubeconform`-валидации;
- retry logic добавлена только к download bootstrap binary, а не размазана по
  всему verify pipeline;
- cache contract не изменён:
  если binary уже лежит в `.cache/kubeconform/...`, сеть не нужна.

## Проверки

- `make kubeconform`
- `make verify`
- `werf config render --dev --env dev controller controller-runtime dmcr`
- `werf build --dev --env dev --platform=linux/amd64 controller controller-runtime dmcr`
- `git diff --check`

## Findings

Blocking findings нет.

## Residual risk

- аналогичные one-shot downloads всё ещё есть в других tooling scripts, но этот
  slice их намеренно не трогает, чтобы не размывать scope.
