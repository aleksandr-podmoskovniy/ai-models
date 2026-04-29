# TASK: Efficient ModelPack compression and resumable distribution design

## Контекст

`ModelPack` уже умеет публиковать model artifacts во внутренний `DMCR` как OCI
artifact. Текущий byte path оптимизирован на streaming/object-source:

- большие weight-файлы публикуются как `raw` layers без compression;
- маленькие архивные слои могут идти как `tar`, `tar+gzip` или `tar+zstd`;
- direct-upload умеет resume по part offset и сохраняет checkpoint;
- materialize сейчас читает каждый registry blob целиком и распаковывает его
  последовательно.

Запрос: спроектировать production-ready схему, которая уменьшит сетевой и
дисковый cost доставки моделей, ускорит materialize/node-cache, не сломает
resume после сбоев и не введёт тяжёлую CPU-bound compression как новый SPOF.

## Scope

- Проанализировать текущий `ModelPack` publish/materialize/direct-upload path.
- Сверить подход с референсами OCI, lazy/chunked distribution и seekable
  compression.
- Спроектировать целевой `ModelPack` layout для:
  - chunked large files;
  - bounded optional compression;
  - resumable source download и registry upload;
  - resumable materialize/node-cache prefetch;
  - observability и storage accounting.
- Сформировать implementation slices без немедленного изменения runtime-кода.

## Non-goals

- Не включать слепой `tar+zstd` для всех моделей.
- Не менять публичный `Model` / `ClusterModel` API в этом slice.
- Не делать lossy quantization или model conversion.
- Не требовать полного локального staging-файла перед публикацией.
- Не ломать существующие `raw` / `tar` ModelPack artifacts.
- Не вводить пользовательские крутилки для compression/chunking без явной
  необходимости.

## Acceptance criteria

- Текущие ограничения `ModelPack` зафиксированы.
- Выбран подход, который сохраняет resume после worker/controller/DMCR restart.
- Compression policy не ухудшает CPU/memory profile для already-compressed или
  low-compressibility weights.
- У плана есть честный ответ, когда compression не включается: startup
  ускоряется parallel chunk pull/resume/dedup, а не фиктивным "сжать любой
  файл".
- План совместим с OCI registry semantics и текущим `DMCR`.
- План различает `storedBytes`, `materializedBytes` и `reservedBytes` по
  паттерну virtualization `STOREDSIZE` / `UNPACKEDSIZE`, но не копирует
  VM-image assumptions в модельный registry.
- Есть implementation-ready `CONTRACT.ru.md` с media types, index schema,
  validation rules, bin-pack policy, checkpoint budget and rollout gates.
- План описывает phased rollout и rollback.
- Реализация разбита на slices с узкими проверками.

## Architecture acceptance criteria

- `ports/modelpack` остаётся contract boundary, а OCI details остаются в
  `adapters/modelpack/oci`.
- Source fetch/upload logic не смешивается с OCI manifest shaping.
- Compression/chunk planning выделяется как policy/domain-level decision, а
  concrete encoding остаётся adapter/dataplane detail.
- Node-cache использует тот же immutable chunk index, а не строит вторую
  cache semantics surface.
- Materializer становится resumable/idempotent per chunk, а не только per
  artifact.

## Orchestration

Mode: `full` for design review.

Reason: work touches storage, runtime, OCI/DMCR integration, node-cache,
observability and HA semantics. Read-only architecture subagents are required
before implementation. Implementation slice also stays `full`.

## Rollback point

До implementation: удалить `plans/active/modelpack-efficient-compression-design`.

После implementation: feature-gate new chunked layout as `Auto`, keep legacy
`raw` / `tar` materialization support and allow controller to publish legacy
layout while chunked layout is disabled.
