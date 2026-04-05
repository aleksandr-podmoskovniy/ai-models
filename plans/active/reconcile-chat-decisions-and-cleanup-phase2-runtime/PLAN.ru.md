# PLAN

## Current phase

Этап 2. Corrective cleanup and runtime alignment before the next release-path
implementation slice.

## Режим orchestration

- mode: `full`
- read-only subagents before code cleanup:
  - runtime/design drift audit against actual chat decisions;
  - active-bundle / stale-folder cleanup audit.

## Локальные audit conclusions before cleanup

- Current repo already holds the correct invariant that runtime consumes only
  `OCI from registry`, but this is not yet reflected consistently in every
  active bundle and runtime-facing wording.
- The main remaining active patchwork is no longer file-level only; it also
  exists as multiple overlapping active bundles with completed slices still
  left under `plans/active/`.
- The next safe cleanup step is:
  1. fix current active wording and runtime-facing error strings;
  2. archive obviously completed active bundles;
  3. only then continue structural runtime cleanup.

## Slice 1. Chat-decision inventory and runtime drift audit

Цель:

- явно зафиксировать, что именно уже agreed and must be treated as project
  invariants.

Изменения:

- current bundle notes
- repo-memory docs if needed

Результат:

- one inventory document mapping actual agreements to current repo state;
- explicit list of remaining drift in runtime/docs/plans.

Проверки:

- targeted grep/manual consistency checks
- `git diff --check`

## Slice 2. Align runtime/docs/skills with agreed invariants

Цель:

- убрать оставшийся drift in runtime/materialization/project-memory wording.

Изменения:

- `.agents/skills/*`
- `images/controller/README.md`
- current runtime/design bundles
- `docs/CONFIGURATION*`
- `api/*` / `crds/*` only if needed

Результат:

- one coherent wording:
  - `ModelPack` contract;
  - implementation adapters;
  - registry/OCI-only runtime input;
  - hidden backend under `DVCR`;
  - init-container/local-path runtime handoff.

Проверки:

- targeted `go test` for touched controller packages if code changes
- `git diff --check`

## Slice 3. Archive stale active bundles and remove dead project patchwork

Цель:

- схлопнуть duplicate workstreams и убрать stale dirs/files.

Изменения:

- `plans/active/*`
- `plans/archive/*`
- dead docs/files/directories proven stale by Slice 1 inventory

Результат:

- reduced active-bundle set with one source of truth per workstream;
- old junk/duplicate folders removed or archived.

Проверки:

- manual bundle inventory
- `git diff --check`

## Slice 4. Continue hard refactor on the clean runtime path

Цель:

- после cleanup продолжить structural refactor only where it clarifies the next
  implementation path.

Изменения:

- `images/controller/internal/*`
- supporting docs/tests/branch matrices

Результат:

- cleaner package boundaries and fewer legacy seams on the runtime path;
- next concrete implementation slice becomes explicit.

Текущий bounded slice внутри этого этапа:

- удалить adapter-local compatibility shims и facade-only tests, которые уже не
  несут отдельного сигнала после hexagonal corrective cuts;
- не трогать release-path пакеты, если файл всё ещё нужен для live publication,
  cleanup или v0 init-adapter rendering;
- считать безопасными target'ами только:
  - redundant alias-only wrappers;
  - low-signal delegation tests;
  - stale materialization/publication shims, у которых уже есть прямой
    domain/ports replacement.

Проверки:

- targeted `go test`
- controller quality gates
- `make verify` if feasible

## Rollback point

После Slice 2 должен уже существовать clean and coherent source of truth even
если structural cleanup will stop before all stale folders are archived.

## Final validation

- controller quality gates
- targeted `go test` for touched packages
- `make verify` if feasible for current repo state
- `git diff --check`
