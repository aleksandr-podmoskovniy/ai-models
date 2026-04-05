# REVIEW

## Что сделано

- `images/controller/werf.inc.yaml` переведён с `builder/alpine` на
  `base/distroless`.
- Финальный runtime shell controller остался минимальным:
  - один бинарь;
  - non-root user `64535`;
  - без shell/runtime мусора.

## Read-only выводы перед изменением

- В репозитории уже есть canonical base image `base/distroless`:
  `build/base-images/deckhouse_images.yml`.
- Controller binary уже собирается как статический Go binary
  (`CGO_ENABLED=0`), поэтому alpine runtime не нужен.
- `templates/controller/deployment.yaml` уже согласован с distroless runtime:
  non-root, HTTP probes, без shell-based assumptions и без writable runtime
  shell.
- Сам controller не делает direct external HTTP/OCI access; этим занимаются
  worker Pods. Поэтому system CA bundle внутри controller image не является
  блокером для этого slice.

## Проверки

- `go test ./...` in `images/controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
- `git diff --check`

## Остаточные риски

- Backend image всё ещё не distroless, но это вне scope этого slice.
- Если позже controller начнёт сам ходить во внешние HTTPS endpoints, вопрос CA
  trust bundle нужно будет проверять уже отдельно для controller runtime shell.

## Следующий кодовый шаг

- Вернуться к controller corrective refactor bundle и продолжить hard-refactor
  по shared `ports/*` extraction из adapter-local пакетов.
- Только после этого идти в следующий feature slice:
  `HF/HTTP authSecretRef`, затем runtime materializer / `kitinit` path.
