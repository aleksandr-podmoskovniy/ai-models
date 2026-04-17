## Findings

- Логи и live код подтверждают, что remote `HuggingFace` path уже использует
  source mirror в object storage под `.mirror`, а затем materialize'ит локальный
  workspace в publish worker. Следовательно, current wording `remote raw ingest`
  вводит оператора в заблуждение.
- Default runtime path переведён на
  `publicationRuntime.workVolume.type=PersistentVolumeClaim`, а generated PVC
  уже использует default `StorageClass`, если `storageClassName` пустой. Это
  убирает implicit ставку на node `ephemeral-storage` как platform default для
  крупных моделей.
- Publish worker по-прежнему требует local materialization; object storage сам
  по себе не устраняет потребность в workspace. Значит одной только “записи в
  S3” недостаточно, нужен явный workspace scenario и guardrail.
- Старый remote raw-stage copy path для `source.url` удалён как мёртвый код.
  При этом `raw/` subtree сохранён как общий namespace для upload staging и
  source mirror prefixes; полный rename этого subtree в данном slice не нужен.

## Missing checks

- Реализация size-aware guardrail ещё не сделана.
- Не предпринималась миграция внутренних runtime flag/env имён вроде
  `raw-stage-bucket` / `raw-stage-key-prefix`; operator-facing wording уже
  выровнено, но internal shell naming всё ещё историческое.

## Residual risk

- Large model всё ещё может упереться в недостаточный локальный workspace уже в
  PVC-backed scenario, просто failure mode теперь меньше зависит от node
  `ephemeral-storage`. Без size-aware guardrail оператор по-прежнему узнаёт об
  exact capacity mismatch слишком поздно.
- Runtime byte-path всё ещё требует локальной materialization копии поверх
  source mirror в object storage; zero-local-copy pipeline в этот slice не
  входит.

## Validation

- `go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`
  в `images/controller`
- `make helm-template`
- `make kubeconform`
- `make verify`
