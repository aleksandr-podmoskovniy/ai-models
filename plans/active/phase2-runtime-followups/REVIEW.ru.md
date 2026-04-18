# REVIEW

## Slice

Slice 8. Land resumable mirror-byte transport.

## Что проверено

- Изменение осталось в scope текущего bundle:
  - `sourcefetch`
  - `sourcemirror`
  - `publishworker`
  - `artifactcleanup`
  - structure/evidence docs
- Новый durable seam не создаёт второй published source of truth:
  - published truth остаётся `DMCR` + immutable OCI `ModelPack`
  - source mirror используется только как ingress/runtime-owned persisted mirror
- Cleanup ownership доведён до конца:
  - delete path удаляет registry metadata prefix
  - delete path удаляет source mirror prefix
- Текущий slice не вернул generic HTTP source и не размазал resume state по
  случайным пакетам.

## Проверки

- `cd images/controller && go test ./internal/adapters/sourcefetch ./internal/dataplane/publishworker ./internal/dataplane/artifactcleanup ./internal/support/cleanuphandle ./internal/ports/sourcemirror`
- `make verify`
- `git diff --check`

## Findings

Blocking findings нет.

## Residual risk

- Mirror transport уже restart-safe, но пока остаётся последовательным:
  - нет parallel download по файлам
  - нет intra-file parallel chunking
  - нет throughput tuning
- Это не ломает correctness текущего slice, но остаётся следующим bounded
  performance/hardening workstream.

## Slice 9. Structural package-map cleanup

### Что проверено

- misleading K8s adapter name больше не конфликтует с реальными storage
  adapters:
  - old: `k8s/objectstorage`
  - new: `k8s/storageprojection`
- rename не меняет ownership:
  пакет остаётся только env/volume projection glue для pod shaping.
- `STRUCTURE.ru.md` снова описывает живое дерево, а не historical rename log,
  и больше не теряет `support/uploadsessiontoken/`.

### Findings

Blocking findings нет.

### Residual risk

- rename сам по себе не уменьшает byte-path complexity и не заменяет
  дальнейший source-ingest hardening;
- следующий structural drift теперь вероятнее всего будет уже не в naming, а в
  разрастании concrete adapter packages, если package map снова перестанут
  держать жёстко.

## Slice 10. Dead shared-result cleanup

### Что проверено

- `internal/ports/publishop` больше не держит мёртвый `Result`, который не
  участвовал в live runtime path;
- shared port package теперь несёт только реально используемые request/status
  contracts, а payload/result responsibility остаётся в:
  - `internal/publishedsnapshot/`
  - `internal/publicationartifact/`
- structural docs больше не описывают ложную общую boundary вокруг третьего
  `Result`.

### Findings

Blocking findings нет.

### Residual risk

- cleanup сам по себе не уменьшает adapter/package count;
- следующий structural drift надо продолжать давить по usage graph, а не по
  чисто текстовым rename-идеям.

## Slice 11. Source-mirror custom-CA trust fix

### Что проверено

- presigned multipart upload path больше не использует `http.DefaultClient`
  вслепую, если upload-staging adapter уже владеет CA-aware HTTP client;
- wiring не размазан через ad-hoc config duplicate:
  источник trust остаётся в S3 adapter, а source-mirror path только
  переиспользует тот же transport;
- regression coverage есть на двух уровнях:
  - custom-CA TLS presigned upload endpoint
  - publishworker wiring of the propagated HTTP client

### Findings

Blocking findings нет.

### Residual risk

- fix закрывает correctness на custom-CA endpoint, но не добавляет throughput
  tuning или file-level parallelism;
- live cluster still needs a fresh rollout before the `Gemma` smoke can be
  expected to pass with this fix.

## Slice 12. Controller entrypoint shell split

### Что проверено

- `cmd/ai-models-controller` больше не держит env contract, quantity parsing и
  bootstrap option shaping в одном oversized `run.go`;
- новые files имеют defendable boundaries:
  - `env.go` — process env contract and pass-through helpers
  - `config.go` — parsed manager config and bootstrap option shaping
  - `resources.go` — publication worker resource/work-volume parsing
  - `run.go` — thin execution flow
- split не меняет runtime semantics controller startup path.

### Findings

Blocking findings нет.

### Residual risk

- split уменьшает monolith, но не заменяет будущую repo-wide structural ревизию
  за пределами controller runtime;
- in-repo runtime delivery boundary уже concrete, но consumer-module adoption
  outside `ai-models` всё ещё остаётся отдельным workstream.

## Slice 16. Runtime delivery wiring

### Что должно быть проверено

- `materialize-artifact` перестаёт быть standalone helper without consumer-side
  wiring and получает reusable K8s adapter boundary;
- stable local model path contract зафиксирован в shared runtime surface, а не
  держится только на implicit knowledge of current `ModelPack` contents;
- read-only DMCR auth/CA projection reuse идёт через существующий
  `ociregistry` seam без нового ad-hoc secret/render shell;
- `ai-models` не invents a fake inference consumer: delivery slice остаётся
  adapter-agnostic and K8s-oriented;
- docs и package map явно отражают, что landed reusable consumer-side wiring
  now exists, while concrete `ai-inference` integration can remain a later
  module-to-module slice.

### Findings

Blocking findings нет.

### Residual risk

- concrete consumer integration в другом модуле всё ещё может потребовать
  thin adapter overlay over this reusable wiring;
- performance/parallelism of source mirror remains a separate workstream and
  deliberately не смешивается с delivery slice.

## Slice 17. OCI materializer internal split

### Что проверено

- `internal/adapters/modelpack/oci/materialize.go` больше не смешивает fresh
  materialization flow, marker-based reuse, destination swap и contract-path
  normalization в одном file;
- новые files имеют defendable boundaries:
  - `materialize.go` — top-level materialization orchestration
  - `materialize_contract.go` — stable local model path contract
  - `materialize_destination.go` — safe destination replacement
  - `materialize_reuse.go` — marker-based reuse
- `images/controller/STRUCTURE.ru.md` остаётся в синхроне с live tree после
  этого split и не превращается в stale refactor diary.

### Findings

Blocking findings нет.

### Residual risk

- split уменьшает локальный monolith pressure, но не заменяет будущую
  concrete consumer integration;
- следующий structural cleanup надо искать уже по usage graph вне `oci/`,
  а не дробить этот package бесконечно.

## Slice 18. Shared bounded-volume contract reuse

### Что проверено

- `k8s/modeldelivery` больше не держит второй local contract для
  `EmptyDir`/`PersistentVolumeClaim` поверх уже существующего
  `k8s/workloadpod`;
- это был исторический intermediate шаг; publication-side shared
  `workloadpod` boundary позже retired, когда sourceworker перестал
  требовать local work-volume contract;
- runtime delivery error messages больше не тащат publication-specific
  `work volume` wording.

### Findings

Blocking findings нет.

### Residual risk

- `modeldelivery` остаётся reusable seam, но concrete consumer integration
  outside this repo всё ещё отдельный workstream;
- следующий cleanup надо искать уже не в duplicated volume shaping, а в
  следующей реально живой usage-graph hotspot.

## Slice 19. Sourceworker pod rendering split

### Что проверено

- `internal/adapters/k8s/sourceworker/build.go` больше не смешивает top-level
  build orchestration, runtime env/volume shaping и source-specific argv;
- новые files имеют defendable boundaries:
  - `build.go` — sourceworker build orchestration
  - `build_runtime.go` — pod/container/env/volume shaping
  - `build_args.go` — source-specific argv shaping
- split не возвращает adapter-local wrappers поверх already-live
  `publishop.Request` и не меняет publish-worker pod semantics.

### Findings

Blocking findings нет.

### Residual risk

- `sourceworker/` остаётся важным K8s runtime hotspot и не должен дальше
  обрастать policy/status logic;
- следующий structural cleanup надо выбирать уже по следующей usage-graph
  hotspot, а не дробить этот package бесконечно.

## Slice 20. Cmdsupport shared process split

### Что проверено

- `internal/cmdsupport/common.go` больше не смешивает env parsing, structured
  logging contract и runtime signal/termination helpers в одном file;
- новые files имеют defendable boundaries:
  - `common.go` — repeated flag plus flagset shell
  - `env.go` — env parsing and pass-through helpers
  - `logging.go` — structured logging contract and bridges
  - `runtime.go` — process signal and termination-log helpers
- split не меняет runtime entrypoint semantics controller/runtime commands.

### Findings

Blocking findings нет.

### Residual risk

- `cmdsupport` остаётся shared process glue и дальше не должен снова знать
  concrete adapters or source-specific runtime semantics;
- следующий cleanup надо искать уже в следующем live hotspot, а не снова
  раздувать `cmdsupport` under a new name.

## Slice 21. KitOps concrete adapter split

### Что проверено

- `internal/adapters/modelpack/kitops/adapter.go` больше не смешивает
  publish/remove orchestration, command/auth shell, Kitfile context prep и
  OCI reference helpers в одном file;
- новые files имеют defendable boundaries:
  - `adapter.go` — top-level publish/remove orchestration
  - `command.go` — command/auth shell
  - `context.go` — Kitfile/context prep
  - `reference.go` — OCI reference and runtime env helpers
- split не меняет current boundary: `KitOps` остаётся concrete pack/push/remove
  shell, а registry truth всё так же живёт в shared `oci` path.

### Findings

Blocking findings нет.

### Residual risk

- `kitops` остаётся thin concrete adapter и не должен снова обрастать
  validation/status logic;
- следующий cleanup надо искать уже в следующем live hotspot, а не дробить
  `kitops` дальше без новой responsibility boundary.

## Slice 22. Catalogmetrics collector split

### Что проверено

- `internal/monitoring/catalogmetrics/collector.go` больше не смешивает
  descriptor shell, Kubernetes list/read paths и per-kind metric emission в
  одном file;
- новые files имеют defendable boundaries:
  - `collector.go` — top-level collector shell
  - `collect.go` — object listing over manager reader
  - `report.go` — phase/info/size metric emission helpers
- split не меняет public metric names, labels или collection semantics.

### Findings

Blocking findings нет.

### Residual risk

- `catalogmetrics` остаётся отдельным runtime observability contract и не
  должен снова растворяться в `bootstrap/` или controller packages;
- следующий structural cleanup надо искать уже в следующем usage-graph
  hotspot, а не продолжать механически дробить collectors без новой
  responsibility boundary.

## Slice 23. Publishworker runtime split

### Что проверено

- `internal/dataplane/publishworker/run.go` больше не смешивает worker-level
  contract shell, HF-specific remote path и profile/publish resolution в одном
  file;
- новые files имеют defendable boundaries:
  - `run.go` — top-level worker contract shell
  - `huggingface.go` — HF remote fetch/publish path
  - `profile.go` — profile resolution and publish handoff
- split не меняет upload path, raw provenance semantics или runtime result
  contract.

### Findings

Blocking findings нет.

### Residual risk

- `publishworker` остаётся центральным dataplane runtime hotspot и не должен
  снова обрастать controller/status policy;
- следующий cleanup надо искать уже в следующем live hotspot, а не дробить
  `publishworker` дальше без новой responsibility boundary.

## Slice 24. Safetensors profile split

### Что проверено

- `internal/adapters/modelprofile/safetensors/profile.go` больше не смешивает
  top-level `Resolve`, checkpoint config parsing/value helpers и capability
  inference в одном file;
- новые files имеют defendable boundaries:
  - `profile.go` — top-level profile orchestration
  - `config.go` — config loading, weight scan and typed value helpers
  - `detect.go` — family/context/precision/quantization/parameter inference
- split не меняет resolved profile fields or inference semantics.

### Findings

Blocking findings нет.

### Residual risk

- `modelprofile/safetensors` остаётся concrete resolver и не должен дальше
  обрастать source-specific policy или runtime delivery concerns;
- следующий cleanup надо искать уже в следующем live hotspot, а не дробить
  profile resolver дальше без новой responsibility boundary.

## Slice 25. Publishstate policy validation split

### Что проверено

- `internal/domain/publishstate/policy_validation.go` больше не смешивает
  policy evaluation, inferred model capability mapping и
  normalization/intersection helpers в одном file;
- новые files имеют defendable boundaries:
  - `policy_validation.go` — top-level policy evaluation
  - `policy_infer.go` — inferred model type and endpoint capability mapping
  - `policy_normalize.go` — normalization and set-intersection helpers
- split не меняет validation reasons, messages or acceptance semantics.

### Findings

Blocking findings нет.

### Residual risk

- `publishstate` остаётся domain boundary и не должен дальше обрастать
  Kubernetes shaping или adapter-specific concerns;
- следующий cleanup надо искать уже в следующем live hotspot, а не дробить
  policy validation дальше без новой responsibility boundary.

## Slice 13. Upload-session service split

### Что проверено

- `internal/adapters/k8s/uploadsession/service.go` больше не смешивает
  orchestration, secret lifecycle и handle/token projection в одном file;
- новые files имеют defendable boundaries:
  - `service.go` — constructor plus `GetOrCreate` orchestration
  - `lifecycle.go` — session secret lifecycle and explicit expiration sync
  - `handle.go` — runtime handle shaping and active token resolution
- split не вернул controller-local обход `uploadsession` seam через прямые
  `Secret` mutations outside adapter package.

### Findings

Blocking findings нет.

### Residual risk

- package стал чище, но сам `uploadsession` boundary остаётся важным K8s
  runtime hotspot и дальше не должен обрастать adapter-local policy;
- следующий cleanup надо продолжать по usage graph, а не превращать текущий
  split в повод для искусственного дробления package.

## Slice 14. Sourcefetch archive/materialization split

### Что проверено

- `internal/adapters/sourcefetch/archive.go` больше не смешивает archive
  dispatch, extraction safety и single-file materialization в одном file;
- новые files имеют defendable boundaries:
  - `archive.go` — input dispatch plus archive entrypoint
  - `archive_extract.go` — tar/zip extraction safety and extracted-root logic
  - `materialize.go` — single-file materialization and file IO helpers
- split не меняет acquisition/runtime semantics `PrepareModelInput`.

### Findings

Blocking findings нет.

### Residual risk

- `sourcefetch/` всё ещё остаётся крупным adapter boundary и дальше не должен
  превращаться в контейнер для format/status policy;
- следующий cleanup уже надо выбирать по usage graph внутри `huggingface.go`
  или на repo-wide structural surface, а не дробить archive path бесконечно.

## Slice 15. Sourcefetch HuggingFace split

### Что проверено

- `internal/adapters/sourcefetch/huggingface.go` больше не смешивает HF info
  API helpers, snapshot orchestration и staging/materialization в одном file;
- новые files имеют defendable boundaries:
  - `huggingface.go` — top-level HF fetch orchestration
  - `huggingface_info.go` — HF info API and repo/revision helpers
  - `huggingface_snapshot.go` — snapshot staging/materialization helpers
- split не меняет current public HF source contract или source-mirror behavior.

### Findings

Blocking findings нет.

### Residual risk

- `sourcefetch/` всё ещё остаётся крупным acquisition boundary и следующий
  cleanup надо уже выбирать по usage graph внутри mirror transport or broader
  repo structure;
- live `Gemma` validation всё ещё требует fresh rollout, этот slice сам по
  себе cluster proof не заменяет.

## Slice 26. Concrete runtime delivery service closure

### Что проверено

- `k8s/modeldelivery` доведён от render-only seam до concrete consumer-side
  K8s service;
- cross-namespace read-only DMCR auth/CA projection реально покрыт focused
  regression tests;
- docs и active bundle больше не описывают runtime delivery как intent-only
  boundary.

### Findings

Blocking findings нет.

### Residual risk

- внешний runtime module всё ещё должен добавить свой thin overlay над этим
  reusable service;
- live cluster proof для конкретного consumer workload остаётся отдельным
  follow-up за пределами этого репозитория.

## Slice 27. User-owned `/data/modelcache` contract

### Что проверено

- `materialize-artifact` теперь умеет работать как cache-root runtime:
  складывает payload в `store/<digest>/model` и обновляет `current`
  symlink;
- `k8s/modeldelivery` больше не invent'ит volume или runtime env:
  он reuses уже смонтированный workload storage at `/data/modelcache`,
  inject'ит только init container и digest rollout annotation;
- `k8s/modeldelivery` теперь topology-aware:
  per-pod mounts и StatefulSet claim templates допускаются, direct shared PVC
  на multi-replica workloads требует `ReadWriteMany`, а shared RWX cache
  inject'ит cache-root single-writer coordination в `materialize-artifact`;
- docs и test evidence больше не описывают старый contract
  `AI_MODELS_MODEL_PATH=<mount>/model` как live runtime delivery API.

### Findings

Blocking findings нет.

### Residual risk

- live rollout и cluster proof для shared RWX coordination ещё не сняты;
- shared cache contract сейчас предполагает один model lineage per mounted
  `/data/modelcache` root; multi-model cohabitation на одном PVC не
  проектировалась и не должна silently считаться supported;
- concrete workload mutation controller всё ещё живёт outside this repo и
  должен использовать этот reusable seam без возврата к ad-hoc volume/env
  patching.

## Slice 30. Controller-owned workload delivery adoption

### Что проверено

- `ai-models` теперь сам доводит runtime delivery до live workload mutation
  path через `internal/controllers/workloaddelivery`;
- user-facing contract остался минимальным:
  только top-level annotations `ai.deckhouse.io/model` и
  `ai.deckhouse.io/clustermodel`;
- generic workload delivery не ушёл в admission webhook surface: mutation
  остаётся controller-driven и opt-in only, а watch scope сузился до
  opt-in/managed workloads плюс referenced `Model` / `ClusterModel`;
- stale managed state очищается, когда annotation исчезает или referenced
  model ещё не `Ready`;
- invalid multi-replica direct PVC topology fail-closed reject'ится без leaked
  projected OCI auth secret;
- direct `Job` больше не маскируется как безопасный controller-mutation path.

### Findings

Blocking findings нет.

### Residual risk

- live cluster proof для annotated workload на shared `RWX` topology ещё надо
  снять после rollout;
- если future work попытается вернуть generic workload delivery в admission
  webhook surface, это уже будет architectural regression relative to current
  safer controller-owned path.

## Slice 31. Fix workload-delivery bootstrap default regression

### Что проверено

- live cluster defect оказался реальным code bug, а не rollout noise:
  новый `ai-models-controller` pod падал на startup с
  `runtime delivery init container name must not be empty`;
- root cause был в том, что `workloaddelivery.Options.Validate()` валидировал
  сырые `modeldelivery.Options`, не прогнав defaults normalization;
- после фикса validation идёт по normalized options, и default
  `InitContainerName` снова совместим с controller bootstrap path.

### Findings

Blocking findings нет.

### Residual risk

- live cluster ещё требует новый rollout, иначе stuck deployment останется на
  старом broken image revision и runtime delivery не дойдёт до практической
  проверки;
- даже после bootstrap fix runtime-delivery path всё ещё не имеет live proof
  на annotated workload topology, только code/test proof.

## Slice 33. Rework HF file policy from a real corpus

### Что проверено

- решение больше не строится на ad-hoc whitelist guesses:
  отдельный corpus note зафиксирован в
  `plans/active/phase2-runtime-followups/HF_FILE_POLICY_CORPUS.ru.md`;
- выборка по `85` HF repos показала, что старый policy false-reject'ил `47`
  поддерживаемых `Safetensors`/`GGUF` repos на harmless side files;
- новая policy оставляет required asset contract прежним, но разводит payload'ы
  на:
  - `keep` для config/tokenizer/chat/module/pooling companions
  - `drop` для docs/eval assets/helper scripts/alternative exports
  - `hard reject` только для compiled/native и archive payload classes;
- upload и HF fetch paths остаются согласованными:
  `modelformat`, `sourcefetch` и `publishworker` теперь тестами подтверждают
  одинаковую семантику strip vs reject.

### Проверки

- `cd images/controller && go test ./internal/adapters/modelformat ./internal/adapters/sourcefetch ./internal/dataplane/publishworker`

### Findings

Blocking findings нет.

### Residual risk

- это всё ещё file-composition policy, а не deep content scanning:
  malicious content внутри harmless-looking text/json payload'ов по-прежнему
  не анализируется на publication stage;
- часть mixed-format repos всё ещё может потребовать отдельной product-policy
  развилки, если later slices захотят deterministic preference между
  `Safetensors` and `GGUF` в одном и том же HF repo.

## Slice 34. Publication/runtime logging transparency

### Что проверено

- изменение осталось в scope текущего bundle и не потащило новый product/API
  contract:
  - shared logging contract в `internal/cmdsupport/*`
  - controller/runtime env wiring
  - publish/materialize/source-fetch/modelpack step logs
  - docs и focused tests;
- controller bootstrap сохранил явный root logger bridge в `slog`,
  `controller-runtime/pkg/log` и `k8s.io/klog/v2`, а runtime entrypoints
  получили единый `LOG_LEVEL` alongside existing `LOG_FORMAT`;
- long-running publication/materialization paths больше не остаются только
  start/finish envelope:
  появились стабильные step-boundary logs для source fetch, upload handling,
  publication profile, `kitops` pack/push, OCI materialization и shared `RWX`
  coordination;
- logging hardening не оставил новый monolith:
  oversized `sourcefetch/huggingface.go` был разрезан обратно после first-pass
  regression на controller complexity gate.

### Проверки

- `cd images/controller && go test ./internal/cmdsupport ./cmd/ai-models-artifact-runtime ./cmd/ai-models-controller ./internal/adapters/k8s/sourceworker ./internal/adapters/k8s/modeldelivery ./internal/controllers/catalogstatus ./internal/dataplane/publishworker ./internal/adapters/sourcefetch ./internal/adapters/modelpack/kitops ./internal/adapters/modelpack/oci`
- `git diff --check`
- `make verify`

### Findings

Blocking findings нет.

### Missing checks

- planned live sanity check на active publish worker/materializer pod не
  выполнен:
  указанный pod `ai-model-publish-6d8b4370-2009-4a74-bfb0-88b30072803b` уже
  отсутствует в кластере, а нового active publish/materialize pod на момент
  проверки не было.

### Residual risk

- после rollout всё ещё нужен один практический smoke:
  поднять новый publish/materialize run и проверить, что на `info` видны
  стабильные step boundaries, а на `debug` появляются дополнительные
  diagnostic fields без log spam;
- separate HF file-policy redesign остаётся следующим bounded slice и не
  закрывается этим logging hardening.
