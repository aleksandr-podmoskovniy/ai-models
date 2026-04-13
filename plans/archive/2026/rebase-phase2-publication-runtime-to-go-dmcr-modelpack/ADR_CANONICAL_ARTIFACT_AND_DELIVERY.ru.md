# ADR: Canonical Artifact And Delivery Boundary

## Статус

Accepted, 2026-04-13.

## Контекст

В текущем обсуждении смешивались четыре разные сущности:

- `HuggingFace` snapshot/checkpoint directory;
- `HuggingFace` local cache (`refs/blobs/snapshots`);
- `MLflow` experiment/model flavor;
- published internal `ModelPack` artifact в `DMCR`.

Из-за этого publication plane начинал выглядеть как поиск "универсального
формата модели". Такого формата нет.

Для serving/runtime нужны не generic source semantics, а один устойчивый
канонический published artifact, который:

- адресуется immutable digest;
- живёт во внутреннем registry;
- не зависит от внешнего `HF` или `MLflow`;
- может быть materialize'нут в обычный local path для runtime.

## Решение

### 1. Канонический published artifact

Канонической published единицей в `ai-models` является:

- immutable OCI artifact в `DMCR`;
- с `ModelPack`-совместимым envelope;
- c payload в виде self-contained runtime-ready model filesystem.

Поддерживаемые canonical families v1:

- `hf-safetensors-v1`
- `gguf-v1`

### 2. Что не является каноническим форматом

Не являются каноническим published format:

- `HuggingFace` local cache layout;
- `MLflow` model flavor / registry representation;
- runtime-specific derivative caches (`npcache`, `tensorizer`, etc.).

Это implementation details отдельных слоёв:

- source acquisition;
- experiment/lineage;
- runtime-local optimization.

### 3. Publication boundary

Publication plane отвечает за:

- source resolution;
- normalization to one of supported canonical families;
- validation/profile extraction;
- packaging;
- publication into immutable OCI digest.

Publication plane не обязан:

- сохранять published artifacts в HF cache layout;
- выдавать runtime-optimized node-local cache;
- тащить runtime delivery обратно к source-specific semantics.

### 4. Delivery boundary

Delivery/runtime plane отвечает за:

- чтение immutable published OCI artifact;
- materialization into local filesystem path;
- optional future runtime-specific cache materialization.

Базовый delivery contract:

- input:
  - immutable `status.artifact.uri`
  - `status.artifact.digest`
  - optional internal family hint
- output:
  - local path on disk
  - marker with digest/family/ready timestamp

Runtime boundary therefore stays:

- `vLLM --model <local-path>`
- `transformers.from_pretrained(<local-path>)`
- future `TGI --model-id <local-path>`

### 5. Роль `HuggingFace` cache

`HF` cache layout полезен как:

- downloader optimization;
- runtime-local optimization for HF-native consumers.

Но он не должен становиться:

- published registry format;
- source of truth for serving.

Future HF-cache materialization is allowed only as a bounded delivery
optimization behind the same delivery seam.

### 6. Роль `MLflow`

`MLflow` остаётся:

- experiment system;
- lineage/metadata system;
- optional upstream producer of candidate outputs.

`MLflow` не является serving registry для phase-2 runtime delivery.

## Последствия

- `status.artifact.uri` / digest остаются главным runtime handoff.
- Появляется отдельный OCI-side materializer boundary `artifact -> local path`.
- `KitOps` остаётся pack/push envelope implementation, но не получает право
  определять delivery semantics.
- Future work делится честно:
  - source-side HF mirror/cache improvements;
  - runtime-side HF-cache materialization;
  - ai-inference wiring.

## Что deliberately не делаем в этом ADR

- не делаем durable HF mirror в этом slice;
- не строим node-local warm caches;
- не wire'им сразу `ai-inference`;
- не расширяем public CRD новыми source/provider trees ради delivery details.
