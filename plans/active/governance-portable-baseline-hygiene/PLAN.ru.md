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

## Validation result

- `find plans/active -maxdepth 1 -mindepth 1 -type d -exec basename {} \; | sort` — OK.
- `make lint-codex-governance` — OK.
- `git diff --check` — OK.
- `make verify` — OK.
