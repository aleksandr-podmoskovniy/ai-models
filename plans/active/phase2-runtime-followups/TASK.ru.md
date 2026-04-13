# Phase-2 Runtime Follow-ups

## Контекст

Предыдущий giant bundle по corrective rebase phase-2 runtime накопил слишком
много закрытой истории и перестал быть удобной рабочей поверхностью.

Он архивирован в:

- `plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/`

Текущий baseline после архивирования:

- phase-2 runtime controller-owned и Go-first;
- canonical published artifact:
  - internal `DMCR`
  - immutable OCI `ModelPack`
- current public source contract:
  - `HuggingFace`
  - `Upload`
- controller structure и test-discipline уже выровнены:
  - package map зафиксирован в `images/controller/STRUCTURE.ru.md`
  - coverage inventory зафиксирован в `images/controller/TEST_EVIDENCE.ru.md`
  - test-file LOC gate уже landed.
- phase-2 live cluster validation уже успела выявить, что second-smoke style
  HF repos могут нести benign alternative export artifacts вроде `onnx/`,
  и это не должно ломать canonical `Safetensors` ingest.
- следующая live validation surface должна проверить current public HF source
  contract на реальном small official `Gemma 4` checkpoint и зафиксировать,
  достаточно ли текущего `source.url=https://huggingface.co/...` для user-facing
  UX до отдельного API cut под `repoID + revision`.
- live `Gemma 4` smoke дополнительно вскрыла целостностный дефект публикации:
  published `ModelPack` дошёл до `Ready`, но в `DMCR` оказался пустой
  weight-layer размером `1024` байта вместо реальных весов модели.
- текущий `HF` ingest path всё ещё restart-unsafe:
  - plain per-file `GET`
  - local `O_TRUNC` writes in pod workspace
  - no `Range` resume
  - no persisted progress state
  - no durable shared mirror before publication.
  Это уже недостаточно для больших моделей и станет общим риском для будущих
  non-HF sources вроде Ollama-like registries.
- после первых corrective slices baseline изменился:
  - persisted source mirror now exists in object storage
  - resumable multipart mirror upload plus HTTP `Range` resume is landed
  - local materialization already reads from the mirror
  - remaining gap is parallelism/throughput, not complete restart-unsafety
- package map тоже потребовала отдельного cleanup:
  - `internal/adapters/k8s/objectstorage` был только env/volume projection
    glue, но назывался как реальный storage adapter;
  - это уже конфликтовало с живыми adapter boundaries:
    `uploadstaging/s3` и `sourcemirror/objectstore`;
  - `STRUCTURE.ru.md` при этом не отражал `support/uploadsessiontoken/` и
    начинал разрастаться обратно в исторический log.
- следующий structural drift вскрылся уже не в naming, а в shared contracts:
  `internal/ports/publishop` всё ещё держал мёртвый `Result`, хотя live tree
  уже использует `publishedsnapshot.Result` и `publicationartifact.Result`.
- live `Gemma 4` smoke после landed source-mirror byte path вскрыл новый
  bounded regression:
  presigned multipart upload в source mirror шёл через `http.DefaultClient`,
  поэтому bypass'ил custom S3 CA trust и падал на
  `x509: certificate signed by unknown authority`.

Нужен новый компактный canonical active bundle, чтобы следующие bounded slices
шли без повторного разрастания `plans/active`.

## Постановка задачи

Перезапустить phase-2 runtime planning surface:

- убрать giant historical bundle из `plans/active`;
- оставить его как инженерную историю в `plans/archive/2026`;
- создать новый короткий active bundle с текущим baseline, открытыми
  workstreams и validation expectations;
- зафиксировать planning hygiene, чтобы следующие active bundles снова не
  превращались в многотысячную летопись.

## Scope

- архивировать старый active bundle;
- создать новый canonical active bundle для следующих phase-2 runtime slices;
- перенести в него только:
  - current baseline
  - active invariants
  - current open workstreams
  - validation rules
- обновить planning hygiene docs, если это нужно для закрепления правила.
- прогнать live smoke на current runtime contract для small official `Gemma 4`
  checkpoint и зафиксировать operational result.
- устранить live integrity defect в `KitOps` publication path, из-за которого
  published `ModelPack` может содержать только пустой layer-shell.
- начать corrective redesign source ingest:
  - вместо transient local-first download
  - в сторону durable source mirror в object storage with resumable byte path.
- выровнять package map controller runtime и удалить live naming collisions.
- вырезать мёртвые shared handoff types, если их responsibility уже живёт в
  более узких live models.
- восстановить custom-CA trust в presigned source-mirror multipart upload path.

## Non-goals

- не менять runtime code, API, values, templates или текущий product scope;
- не делать в этом срезе отдельный API redesign под `source.repoID` /
  `source.revision`, если live smoke укладывается в current contract;
- не переделывать сейчас весь `ModelPack`/OCI contract или delivery path за
  пределами bounded corrective fix для real-content publication;
- не пытаться в одном срезе довести весь resumable downloader до production
  parity вместе с `Range`, multipart resume, scheduling и runtime delivery;
  parallelism и throughput tuning остаются отдельным slice;
- не переписывать архивированный bundle задним числом;
- не дробить историю на несколько новых active bundles одновременно;
- не переносить в новый bundle весь старый review log и slice-by-slice history.

## Затрагиваемые области

- `plans/active/*`
- `plans/archive/2026/*`
- `plans/README.md`
- live cluster `Model` smoke surface
- `images/controller/internal/ports/*`
- `images/controller/internal/adapters/*`
- `images/controller/STRUCTURE.ru.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## Критерии приёмки

- giant bundle больше не лежит в `plans/active`;
- в `plans/active` остаётся один компактный canonical bundle для phase-2
  runtime continuation;
- новый bundle явно ссылается на archived predecessor как на историю, а не
  дублирует её;
- planning hygiene rule про oversized active bundles зафиксирован в repo docs;
- layout `plans/active` и `plans/archive` остаётся чистым и понятным.
- current public `HuggingFace` source contract проверен живьём на official
  small `Gemma 4` checkpoint;
- по результату есть готовый working manifest или зафиксированный bounded
  defect с фактами из live cluster.
- опубликованный `ModelPack` после live smoke содержит реальные model bytes, а
  не пустой tar layer или symlink shell;
- corrective regression не допускает возврата symlink-based `kitops` packing
  context.
- durable source mirror direction зафиксирован как отдельный live seam:
  - manifest/state persisted outside pod
  - local workspace перестаёт быть единственным download truth
- landed slices уже довели seam до resumable multipart mirror bytes without
  architectural patchwork.
- misleading package names больше не конфликтуют с уже существующими live
  boundaries и `STRUCTURE.ru.md` отражает текущее дерево без выпавших support
  packages.
- в shared port packages больше не остаётся мёртвых result-wrapper types,
  которые не участвуют в live runtime path и только создают ложную общую
  boundary.
- presigned multipart upload path для source mirror использует тот же
  CA-aware HTTP trust contract, что и основной S3 adapter, и не ломается на
  custom-CA object-storage endpoint.

## Риски

- можно потерять текущий baseline, если новый bundle будет слишком пустым;
- можно оставить параллельные active bundles и снова получить split-brain;
- можно случайно дублировать в новом bundle старую историю вместо новой
  компактной рабочей поверхности.
- live smoke может занять заметное время из-за размера модели и скрыть, где
  именно находится bottleneck: source ingest, publish, registry или cleanup.
- быстрый "фикс" через дополнительную полную локальную копию модели перед
  `kit pack` может ухудшить byte path и ещё сильнее поднять storage pressure.
- если source-mirror seam будет размазан по `sourcefetch`, `uploadstaging` и
  `publishworker` без явного port/adapter split, следующий этап быстро снова
  превратится в монолит.
- misleading package names могут снова замаскировать реальные boundaries и
  привести к следующему structural drift при первом же новом adapter slice.
- presigned upload path может снова silently bypass'ить storage trust
  settings, если HTTP client wiring останется локальной деталью S3 adapter
  без отдельной regression coverage.
