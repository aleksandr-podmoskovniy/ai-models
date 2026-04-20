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
- после landing controller-owned workload delivery и RWX coordination ещё не
  закрыт live proof на большом checkpoint и multi-replica shared-cache
  topology:
  - нужен отдельный smoke на `google/gemma-3-12b-it`;
  - нужен explicit path `HuggingFace -> DMCR -> runtime`;
  - нужен факт, что `ReadWriteMany` cache корректно обслуживает `3` реплики
    одного workload без повторного полной materialization per replica и без
    race на shared cache root.
- live попытка взять `google/gemma-3-12b-it` показала более широкий contract
  gap, чем single-repo exception:
  - current `Safetensors` file policy reject'ит `chat_template.json`;
  - auto-detect branch падает на той же раскладке, потому что для `GGUF`
    reject'ится `added_tokens.json`;
  - значит текущий format policy слишком узкий для реального `Hugging Face`
    corpus и требует evidence-based redesign вместо точечных whitelist fixes.
- отдельный live symptom вскрылся на `google/gemma-4-E4B-it`:
  - publish worker может долго оставаться в `Publishing`;
  - worker pod логирует только старт и терминальный результат;
  - текущая publication/runtime observability недостаточна для эксплуатации и
    для локализации bottleneck между source-info, file selection, mirror,
    materialization, pack/push и inspect.

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
- довести runtime delivery до reusable consumer-side wiring:
  - stable local model path contract
  - read-only DMCR auth projection reuse
  - concrete `PodTemplateSpec` mutation and init-container wiring for
    `materialize-artifact`
- прогнать live RWX smoke на большом official checkpoint
  `google/gemma-3-12b-it` и зафиксировать фактическое поведение shared cache на
  `3` репликах.
- собрать corpus по реальным `Hugging Face` repos и на его основе пересмотреть
  file policy:
  - что keep;
  - что benign drop;
  - что должно оставаться hard reject.
- довести publication/runtime logging до прозрачного contract:
  - единый log-level control;
  - явные progress logs по основным step boundaries;
  - live-observable fields, достаточные для локализации торможения и отказа.
- разрезать oversized controller entrypoint shell, если `cmd/` снова начинает
  смешивать env contract, resource parsing и bootstrap wiring в одном файле.
- держать `cmd/ai-models-controller` defendable как thin shell после недавних
  runtime/logging slices, а не как новый config monolith.
- не оставлять `internal/adapters/k8s/uploadsession/service.go` местом, где
  снова смешаны:
  - session secret lifecycle
  - stale-secret recovery
  - explicit expiration sync
  - upload handle/token projection.
- не оставлять `internal/adapters/sourcefetch/archive.go` местом, где
  одновременно живут:
  - archive dispatch
  - extraction safety
  - extracted-root normalization
  - single-file materialization
  - GGUF/file IO helpers.
- не оставлять `internal/adapters/sourcefetch/huggingface.go` местом, где
  одновременно живут:
  - HF model info API helpers
  - mirror-or-local snapshot acquisition orchestration
  - snapshot staging/materialization.

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
- `images/controller/README.md`
- `docs/README.md`
- `docs/README.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

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
- current public `HuggingFace` source contract дополнительно проверен живьём на
  большом official checkpoint `google/gemma-3-12b-it` без fallback на upload
  path.
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
- consumer-side runtime delivery больше не остаётся только intent в docs:
  concrete reusable K8s `PodTemplateSpec` mutation service над
  `materialize-artifact` landed, stable local cache-root contract
  `/data/modelcache/current` зафиксирован, а read-only DMCR auth/CA
  projection reused без нового ad-hoc shell и с cross-namespace
  runtime-namespace delivery.
- runtime delivery topology должна оставаться fail-closed:
  per-pod storage и StatefulSet claim templates допускаются, direct shared PVC
  на multi-replica workloads должен требовать `ReadWriteMany`, а shared RWX
  cache обязан координировать одного writer прямо на shared cache root.
- large-model live smoke должен подтверждать или опровергать этот контракт
  фактами из кластера:
  - `RWX` cache mounted into one workload with `3` replicas;
  - runtime path materializes large model into shared cache root;
  - follower replicas не ломают shared cache и не расходятся по digest/state;
  - если полная single-download semantics не выполняется, bounded defect
    зафиксирован с фактами из logs/events/runtime filesystem.
- file policy больше не строится на ad-hoc guesses:
  - есть зафиксированная выборка `Hugging Face` repos из разных семейств и
    форматов;
  - common metadata/chat/tokenizer files разведены на `keep` vs benign `drop`;
  - hard rejects сужены до реально опасных или принципиально unsupported
    artefact classes.
- publication/runtime logging больше не остаётся только старт/финиш envelope:
  - есть единый `LOG_LEVEL`-style contract;
  - normal `info` показывает step transitions и ключевые counters/identifiers;
  - `debug` можно включить для более подробной step-local диагностики без
    изменения кода и без ad-hoc `fmt.Printf`;
  - live cluster symptoms вроде долгого `Publishing` можно локализовать по
    логам одного worker pod без ручного перебора внутренних функций.
- touched delivery/runtime code не должен оставлять новый локальный монолит в
  `internal/adapters/modelpack/oci/materialize.go` и не должен снова уводить
  `STRUCTURE.ru.md` от реального package tree.
- reusable `modeldelivery` seam не должен тащить второй bounded-volume
  contract рядом с already-live runtime delivery/publication storage semantics;
- reusable `modeldelivery` seam не должен автоматически invent storage:
  workload сам предоставляет mount на `/data/modelcache`, а delivery wiring
  inject'ит только `materialize-artifact`, projected OCI auth/CA и digest
  rollout annotation без runtime env patching.
- concrete controller-owned workload delivery path должен принять этот seam
  без новых user-facing knobs: только top-level annotations
  `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel`,
  только mutable workload templates, только user-provided `/data/modelcache`
  storage, и без ложной поддержки прямых `Job`.
- generic workload delivery не должен превращаться в cluster-wide admission
  choke point: никаких blocking mutating/validating hooks на чужие workload
  kinds, только controller-driven opt-in adoption, узкий watch scope по
  opt-in/managed workloads и reverse reconcile от `Model` / `ClusterModel`.
- live rollout текущего workload-delivery slice не должен ломать bootstrap:
  delivery defaults, на которые рассчитывает controller config, обязаны
  проходить validation до старта manager, иначе новый controller pod не
  сможет подняться и phase-2 runtime останется на старом rollout.
- `sourceworker/build.go` не должен оставаться следующей oversized pod-rendering
  точкой, где вместе живут orchestration, env shaping, volume shaping и
  source-specific argv.
- `cmdsupport/common.go` после logging hardening не должен оставаться shared
  process-level god-file, который одновременно держит env, runtime signal glue
  и structured logging contract.
- `modelpack/kitops/adapter.go` не должен оставаться следующей oversized
  concrete adapter entrypoint, где смешаны publish/remove orchestration,
  command/auth shell, Kitfile context prep и OCI reference helpers.
- `internal/monitoring/catalogmetrics/collector.go` не должен оставаться
  collector-local god-file, где вместе живут descriptor shell, Kubernetes list
  paths и per-kind metric emission.
- `internal/dataplane/publishworker/run.go` не должен оставаться dataplane-local
  god-file, где вместе живут worker contract shell, HF-specific remote path и
  profile/publish resolution.
- `internal/adapters/modelprofile/safetensors/profile.go` не должен оставаться
  resolver-local god-file, где вместе живут `Resolve`, checkpoint config
  parsing/value helpers и capability inference.
- `internal/domain/publishstate/policy_validation.go` не должен оставаться
  domain-local god-file, где вместе живут policy evaluation, inferred model
  capability mapping и normalization/intersection helpers.

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
- если oversized `cmd/*` shell не резать вовремя, он снова станет входной
  точкой для случайного policy/config drift под видом harmless wiring.
