# Plan

## Current phase

Этап 2: public catalog API (`Model` / `ClusterModel`) и controller-owned
publication/runtime metadata already landed; нужен contract cleanup, чтобы
phase-2 metadata semantics соответствовали internal ADR по `ai-models` и
`ai-inference`.

## Orchestration

`solo`

Задача multi-area и архитектурная, но boundary уже явно задана repo-local
кодом и internal ADR. В текущем рабочем режиме не создаю implicit delegation:
нужный signal можно получить прямым diff review against ADR and repo tests.

## ADR baseline

Выровняться по:

- `internal-docs/2026-03-18-ai-models-catalog.md`
- `internal-docs/2026-03-18-ai-inference-service.md`

Ключевой semantic split:

- `ai-models` публикует platform-facing model metadata;
- `ai-inference` выбирает inference runtime implementation;
- distributed topology (`KubeRay` и подобное) не должна masquerade as
  `Model` runtime compatibility enum.
- public conditions должны оставаться минимальным usability contract, а не
  зеркалом внутренних controller stages.
- AI platform naming должен быть единым: API group и related platform-facing
  prefixes целятся в `ai.deckhouse.io`, а не в module-specific
  `ai-models.deckhouse.io`.

## Slices

### Slice 1. Reframe public API and naming surface

- Цель:
  - убрать `KServe` / `KubeRay` из public runtime enum;
  - сузить `runtimeHints` до реально нужных publication-time hints;
  - выровнять endpoint/runtime wording в API types;
  - сузить public condition types до минимального ADR-aligned набора.
  - переименовать public API group и generated CRD names на `ai.deckhouse.io`;
  - выровнять scripts/RBAC/docs against the same group.
- Файлы:
  - `api/core/*`
  - `api/core/v1alpha1/*`
  - `api/scripts/*`
  - `crds/*`
  - `templates/controller/*`
  - `api/README.md`
- Проверки:
  - `cd api && go test ./...`
  - `cd api && ./scripts/verify-crdgen.sh`
- Артефакт:
  - API types and validations reflect ADR semantics.
  - platform naming surface no longer mixes `ai-models.deckhouse.io` and
    `ai.deckhouse.io`.

### Slice 2. Align publication profile and status projection

- Цель:
  - строить semantic endpoint metadata;
  - убрать guessed runtime compatibility from `runtimeHints.engines`;
  - проецировать в `status.resolved` только defendable metadata;
  - упростить publish/delete status projection вокруг минимального condition
    набора и итогового `Ready`.
  - выровнять controller-owned labels/annotations/finalizers/index fields под
    новый unified prefix там, где это часть live platform contract.
- Файлы:
  - `images/controller/internal/adapters/modelprofile/*`
  - `images/controller/internal/publishedsnapshot/*`
  - `images/controller/internal/dataplane/publishworker/*`
  - `images/controller/internal/domain/publishstate/*`
  - `images/controller/internal/application/publishplan/*`
  - `images/controller/internal/adapters/k8s/sourceworker/*`
  - `images/controller/internal/controllers/workloaddelivery/*`
  - `images/controller/internal/controllers/catalogcleanup/*`
  - `images/controller/internal/support/*`
- Проверки:
  - `cd images/controller && go test ./internal/application/publishplan ./internal/domain/publishstate ./internal/adapters/modelprofile/... ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker ./internal/controllers/workloaddelivery ./internal/controllers/catalogcleanup ./internal/support/...`
- Артефакт:
  - controller metadata path no longer mixes runtime brands and topology.
  - status/conditions path no longer exposes decorative internal stages.
  - controller naming surface no longer keeps stale module-specific prefix.

### Slice 3. Align docs, evidence, and ADR

- Цель:
  - закрепить новый contract в repo-local docs, test evidence и internal ADR.
- Файлы:
  - `docs/CONFIGURATION.md`
  - `docs/CONFIGURATION.ru.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- Проверки:
  - `rg -n "ai-models\\.deckhouse\\.io|KServe|KubeRay|OpenAIChatCompletions|OpenAICompletions|runtimeHints.engines" docs images/controller api`
- Артефакт:
  - repo docs explain the same semantics as code and ADR.
  - ADR examples and field descriptions match live API/status semantics.

## Rollback point

После Slice 1 можно откатиться к текущему contract, не оставив hybrid API +
controller split. После начала Slice 2 API types and controller projection
must stay aligned together.

## Final validation

- `cd api && go test ./...`
- `cd images/controller && go test ./internal/application/publishplan ./internal/domain/publishstate ./internal/adapters/modelprofile/... ./internal/dataplane/publishworker ./internal/adapters/k8s/sourceworker`
- `make verify`
