# План: portable baseline hygiene

## Current phase

Governance task. Она не относится к product phase 1/2/3 напрямую и не должна
менять runtime/API поведение.

## Orchestration

Режим `solo`: это governance/doc-only tightening. По правилам `AGENTS.md`
такие задачи могут оставаться solo, если цель — consistency review surfaces,
а не проектирование нового runtime/API behavior. Subagents в текущем turn не
были явно разрешены.

## Touched Instruction Surfaces

- `.codex/README.md`
- `.codex/governance-inventory.json`
- `.agents/skills/task-intake-and-slicing/SKILL.md`
- `.agents/skills/review-gate/SKILL.md`
- `.codex/agents/task-framer.toml`
- `.codex/agents/repo-architect.toml`
- `.codex/agents/reviewer.toml`
- `plans/README.md` только как существующий источник правил, без правки если
  текущего текста достаточно

## Reusable Core vs Project-Specific Overlay

- Reusable core: task intake, review gate, DKP module shell, config contract,
  runtime integration, K8s API design, controller architecture discipline,
  controller runtime implementation, core read-only/write agent roles,
  active bundle hygiene, baseline porting discipline.
- Project-specific overlay: `ai-models-backend-platform`, `model-catalog-api`,
  `backend_integrator`, ai-models phase narrative and Model/ClusterModel
  details.

## Slices

1. Зафиксировать bundle и active hygiene decision. — выполнено
   - Файлы: `TASK.ru.md`, `PLAN.ru.md`.
   - Проверка: manual consistency with `AGENTS.md` and `plans/README.md`.

2. Заархивировать завершённые и одноразовые active bundles. — выполнено
   - Файлы: `plans/active/*`, `plans/archive/2026/*`.
   - Проверка: `find plans/active -maxdepth 1 -type d`.

3. Усилить reusable skills/agents. — выполнено
   - Файлы: `.agents/skills/task-intake-and-slicing/SKILL.md`,
     `.agents/skills/review-gate/SKILL.md`,
     `.codex/agents/task-framer.toml`, `.codex/agents/repo-architect.toml`,
     `.codex/agents/reviewer.toml`.
   - Проверка: manual consistency against `.codex/README.md`.

4. Обновить README/inventory guardrails. — выполнено
   - Файлы: `.codex/README.md`, `.codex/governance-inventory.json`.
   - Проверка: `make lint-codex-governance`.

5. Review gate и финальная проверка. — выполнено
   - Файлы: `REVIEW.ru.md`, `PLAN.ru.md`.
   - Проверки: `git diff --check`, `make lint-codex-governance`,
     `make verify` если текущее состояние repo позволяет.

## Rollback point

Вернуть архивированные bundles из `plans/archive/2026/*` обратно в
`plans/active/*` и откатить wording changes в skills/agents/README/inventory.

## Final validation

- `find plans/active -maxdepth 1 -type d | sort`
- `make lint-codex-governance`
- `git diff --check`
- `make verify`

## Archived bundles

- `cleanup-without-delete-jobs`
- `ci-dmcr-image-consistency`
- `dmcr-zero-trust-ingest`
- `dmcr-auth-virtualization-parity`
- `dmcr-tls-virtualization-parity`
- `governance-rbac-coverage-discipline`
- `governance-surface-sync`
- `hf-publication-e2e-validation`
- `internal-docs-catalog-refresh`
- `live-cluster-error-triage`
- `module-rootca-virtualization-parity`
- `node-local-cache-runtime-delivery`
- `phase2-dmcr-gc-coalescing`
- `phase2-model-distribution-architecture`
- `publication-gc-operator-followups`
- `publication-network-qos-design`
- `rbac-access-level-coverage`
- `repo-code-reduction-audit`
- `research-kuberay-vllm-v100-gpudirect-inference`
- `rewrite-skala-sdn-rdma-smoke`
- `runtime-baseline-prod-reset`
- `runtime-distribution-observability`
- `runtime-placement-virtualization-style`
- `virtualization-tls-parity-cleanup`
- `verify-complexity-cleanup`

## Retained active shortlist

- `governance-portable-baseline-hygiene`: текущая governance surface cleanup.
- `phase1-gc-sweep-and-fast-seal`: ближайший stage-1 corrective surface по
  storage/GC safety и publication sealing.
- `runtime-delivery-admission-apply`: delivery architecture tail.
- `runtime-delivery-log-contract`: worker/materializer log retention и
  operator-facing log contract.
- `source-fetch-contract-rename`: cleanup public source-fetch contract naming.
- `upload-progress-status`: upload/public status UX hardening.
- `upload-session-bearer-auth`: security hardening для upload-session token
  handoff.

## Result

- `plans/active` очищен от завершённых/reviewed TLS/auth/RBAC/code-reduction,
  live/e2e/placement/gc-followup/complexity bundles, phase-2 design backlog и
  одноразовых research/smoke surfaces; история сохранена в
  `plans/archive/2026`.
- В active оставлен короткий executable shortlist из 7 bundles вместо 31.
- Reusable skills и core agents теперь явно требуют, чтобы active был
  executable work surface, а перенос в соседний DKP module начинался с
  baseline-porting bundle.
- `.codex/README.md` описывает active hygiene и отдельный porting path для
  sibling modules вроде `ai-inference`.
- `.codex/governance-inventory.json` закрепляет новые guardrails как
  machine-checkable требования.

## Continuation 2026-04-26: active bundle review

### Decision

Текущий `plans/active` снова был смесью completed implementation reports,
live-audit/live-ops notes и одного настоящего следующего workstream.

Оставить active:

- `publication-runtime-chaos-resilience`: это не закрытый отчёт, а следующий
  executable resilience/evidence workstream. Он содержит failure matrix,
  stop conditions и rollback для будущего controlled chaos execution.

Архивировать:

- `ai-models-helm-secret-type-recovery`: hook/fix уже реализован, проверка
  описана в bundle, это закрытая recovery history.
- `controller-structure-architecture-parity`: `STRUCTURE.ru.md` и связанные
  guardrails актуализированы, bundle стал historical architecture report.
- `gemma-small-e2e-runtime-audit`: live happy-path trace завершён и findings
  перенесены в последующие workstreams.
- `governance-portable-baseline-hygiene`: после текущего continuation должен
  уйти в архив, чтобы сам governance cleanup не висел как active.
- `k8s-ceph-rbd-stale-lock-cleanup`: live Ceph/RBD cleanup завершён, systemic
  finding зафиксирован как storage-side issue, не active ai-models workstream.
- `multi-model-workload-delivery`: multi-model contract implemented and
  verified; remaining stability work belongs to chaos/resilience or future
  focused implementation bundles.
- `node-local-cache-runtime-delivery`: giant historical node-cache/cutover log,
  live code/docs уже содержат SharedDirect/CSI/runtime baseline; future work
  must start from compact continuation, not this oversized bundle.
- `rbac-role-coverage-hardening`: RBAC parity implemented and validated.
- `runtime-delivery-admission-apply`: admission scheduling gate slice
  implemented and reviewed.
- `runtime-delivery-log-contract`: log/event dedupe implemented and validated.
- `source-fetch-contract-rename`: sourceFetchMode hard-cut rename is reflected
  in code, OpenAPI, templates and docs.
- `upload-progress-status`: top-level status progress is present in API,
  projection and tests.
- `upload-session-bearer-auth`: raw bearer is out of public status; token
  handoff Secret and Bearer-only runtime contract are present.

### Skills/agents update

- `task-intake-and-slicing` must classify active bundles before opening a new
  one and must record active disposition for governance/handoff tasks.
- `review-gate` must treat a completed current bundle left in `plans/active`
  as a finding unless it contains an explicit next executable slice.
- `task_framer`, `repo_architect` and `reviewer` must enforce the same rule at
  role level.
- `.codex/README.md` and `.codex/governance-inventory.json` must keep the
  active-hygiene guardrail machine-checkable.

### Validation for continuation

- `find plans/active -maxdepth 1 -mindepth 1 -type d -exec basename {} \; | sort` — OK, only `publication-runtime-chaos-resilience`.
- `make lint-codex-governance` — OK.
- `git diff --check` — OK.

## Validation result

- `find plans/active -maxdepth 1 -mindepth 1 -type d -exec basename {} \; | sort` — OK.
- `make lint-codex-governance` — OK.
- `git diff --check` — OK.
- `make verify` — OK.
