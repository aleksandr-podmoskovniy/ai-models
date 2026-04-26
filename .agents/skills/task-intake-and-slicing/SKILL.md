---
name: task-intake-and-slicing
description: Use first for any non-trivial request. Converts a human request into a task bundle under plans/active/<slug>/ with scope, non-goals, acceptance criteria, slices, validations, and rollback point.
---

# Task intake and slicing

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. `docs/development/TASK_TEMPLATE.ru.md`
5. `plans/README.md`

## Use this skill when

- the request is larger than one focused code change;
- the request changes architecture, module shell, internal backend integration, or public API;
- the request is still fuzzy and needs to be turned into an executable task.

## Workflow

1. Define the current project phase.
2. Restate the user request in platform terms.
3. Write explicit scope and non-goals.
4. Write acceptance criteria that can actually be checked.
5. If the task touches controller/runtime architecture, include explicit architecture acceptance criteria:
   - use case / port / adapter split
   - max LOC if relevant
   - complexity/quality-gate expectations
   - test evidence shape
6. If the task changes DKP user-facing API, auth/exposure, RBAC templates, or runtime entrypoints, include DKP user-facing RBAC coverage:
   - access levels/personas
   - namespaced vs cluster-wide scope
   - allowed verbs
   - intentional deny paths
   - validation evidence
7. Split the work into slices with concrete file areas.
8. Add validation commands per slice.
9. Add one rollback point.
10. Decide orchestration mode: `solo`, `light`, or `full`.
11. If the task is not `solo`, name the read-only subagents that should review it before implementation.
12. If the task touches repo-local workflow governance (`AGENTS.md`, `.codex/*`, `.agents/skills/*`, `.codex/agents/*`, `docs/development/CODEX_WORKFLOW.ru.md`, `docs/development/TASK_TEMPLATE.ru.md`, `docs/development/REVIEW_CHECKLIST.ru.md`, `plans/README.md`), make it a dedicated governance bundle and add explicit consistency acceptance criteria across all touched instruction layers.
13. If the task tightens the reusable governance baseline, state explicitly what remains reusable core and what stays project-specific overlay.
14. If the task ports this baseline into another repo, record:
   - source repo baseline
   - copied reusable core
   - overlays to replace or remove
   - repo docs that must be rewritten before the first product slice
15. For sibling DKP module work, open a baseline-porting bundle first; do not begin a product/runtime slice until the target repo has replaced source-specific overlays and rewritten repo-specific docs.
16. Before creating a new active bundle, inspect `plans/active` and classify every active bundle as one of:
   - keep: next executable workstream with a concrete next slice;
   - merge: same workstream as the current request;
   - archive: completed review, live-audit, live-ops, research, historical log, or oversized context bundle;
   - delete: only for empty/accidental scaffolding with no engineering record.
17. If active contains completed or historical bundles, archive them before adding new work unless the user explicitly asked only for read-only planning.
18. For governance, handoff, or plan-hygiene tasks, record an `Active bundle disposition` section in the current `PLAN.ru.md`: kept bundles, archived bundles, and why each kept bundle remains executable.
19. If the current task finishes, do not leave its bundle in `plans/active` unless it contains an explicit next executable slice; move the closed bundle to `plans/archive/<year>/`.
20. If the task defines durable project discipline, encode it in repo-local skills or skill references instead of leaving it only in the current bundle.
21. Avoid duplicate active slugs for the same workstream, reuse the current canonical active bundle when the request is a continuation, and archive stale/finished or oversized active bundles when active context drifts.
22. Save the result in `plans/active/<slug>/TASK.ru.md` and `plans/active/<slug>/PLAN.ru.md`.

## Output

A task bundle that another agent can execute without guessing what the user meant.
