# Plan: Runtimehealth workload kind collapse

## 1. Current phase

Работа относится к Phase 1/2 runtime hardening and codebase slimming. Изменение
только в monitoring adapter boundary.

## 2. Orchestration

Режим: `full`.

Причина: исходный запрос требует пройтись по проекту, сравнивать с DKP /
virtualization-style boundaries и использовать subagents. Перед изменением
кода выполнены read-only scans.

Read-only subagents:

- `repo_architect` — выбрать следующий package-local slimming slice без
  boundary drift.
- `backend_integrator` — проверить, что backend-specific candidates не надо
  смешивать с monitoring slice.
- `integration_architect` — проверить runtime/storage/observability risks and
  выбрать безопасный executable slice.

Subagent conclusions:

- `repo_architect`: runtimehealth workload-delivery kind counting is a safe
  package-local optional slice; uploadsessionstate is another good future
  target, but should not be mixed with this work.
- `backend_integrator`: DMCR direct-upload token codec is a separate future
  backend-only candidate; no backend/publication changes should be folded into
  runtimehealth.
- `integration_architect`: runtimehealth contraction is safe if metric names,
  labels and runtime behavior remain unchanged. Larger risks are recorded for
  later: `SharedDirect` readiness gating and upload ingress/controller identity
  split.

## 3. Active Bundle Disposition

- `model-metadata-contract` — keep. Отдельный active workstream по public
  metadata/status contract and future `ai-inference`.
- `runtimehealth-workload-kind-collapse` — current. Единственный write scope:
  package-local runtimehealth workload-kind counting cleanup.

## 4. Slice 1. Workload kind scanner

Цель:

- replace four repeated workload list/count functions with a single local
  workload kind scanner;
- keep typed Kubernetes lists and exact PodTemplate extraction per kind;
- preserve metric contract.

Файлы:

- `images/controller/internal/monitoring/runtimehealth/workload_delivery.go`
- adjacent tests only if existing coverage is insufficient.

Проверки:

- `cd images/controller && go test ./internal/monitoring/runtimehealth`
- `cd images/controller && go test -count=3 ./internal/monitoring/runtimehealth`
- `cd images/controller && go test -race ./internal/monitoring/runtimehealth`
- `git diff --check`

Evidence:

- Implemented candidate was rejected after final reviewer: net LOC win was only
  `284 -> 280`, while the table/type-switch shape weakened typed workload
  extraction and could silently skip a future kind mismatch.
- Runtimehealth code change was reverted. The current bundle is archived as a
  rejected slice rather than kept as pseudo-progress.
- `cd images/controller && go test ./internal/monitoring/runtimehealth`

## 5. Slice 2. Review gate

Цель:

- проверить diff против TASK scope;
- убедиться, что active bundle не превращается в historical log;
- archive current bundle or record next executable slice.

Проверки:

- `review-gate` — completed locally.
- final `reviewer` — completed; medium finding accepted, implementation
  reverted.
- bundle archived as rejected; no next executable slice recorded here.

## 6. Deferred Risks

- `SharedDirect` gating must require ready node-cache runtime, not just any
  selected node. Needs separate runtime behavior slice.
- Public upload ingress currently shares controller Pod/ServiceAccount identity.
  Needs separate API/RBAC/template slice.
- Node-cache desired artifacts should eventually move away from scanning Pod
  annotations directly. Needs separate runtime contract slice.

## 7. Rollback Point

Rollback: revert changes under
`images/controller/internal/monitoring/runtimehealth/` and this plan bundle.
No API, RBAC, templates, runtime entrypoints or metric names are in scope.
