---
name: review-gate
description: Use at the end of any substantial task. Reviews the diff against the current task bundle, stage, repo rules, validation requirements, and long-term maintainability.
---

# Review gate

## Read first

1. `AGENTS.md`
2. `docs/development/REVIEW_CHECKLIST.ru.md`
3. current `plans/active/<slug>/TASK.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- implementation is complete or nearly complete;
- you need a final structured review before handing the result over.

## Workflow

1. Compare the diff against the agreed scope.
2. Look for architecture drift and patchwork symptoms.
3. Check docs, build files, templates, and code for consistency.
4. Check whether the right validations were actually run.
5. Check plan hygiene:
   - duplicate active slugs
   - stale bundles left in `plans/active`
   - current change landed in the correct canonical active bundle and did not create a parallel source of truth
   - implementation drift not reflected in the current bundle
6. If repo-local workflow surfaces changed (`AGENTS.md`, `.codex/*`, `.agents/skills/*`, `.codex/agents/*`, `docs/development/CODEX_WORKFLOW.ru.md`, `docs/development/TASK_TEMPLATE.ru.md`, `docs/development/REVIEW_CHECKLIST.ru.md`, `plans/README.md`), check them as one instruction system:
   - no lower-level file contradicts a higher-level rule
   - skill and agent responsibilities are still distinct
   - no new governance surface was introduced without a real need
   - the task bundle explicitly captured that governance scope
   - reusable core stays portable, and module-specific doctrine stays in
     explicit overlays instead of leaking into generic core
   - if this is a baseline-porting task, the bundle explicitly lists source
     baseline, replaced overlays, and rewritten repo-specific docs
7. If controller code changed, check whether controller quality gates and corrective architecture rules were respected.
   - treat ambiguous package naming such as `app` vs `application` as a real
     finding, not a cosmetic nit
   - treat misleading verify output or controller checks hidden behind broader
     shells as a real finding
   - treat public API noise as a real finding:
     fixed internal output formats, backend entity names, and adapter-specific
     transport encodings must not leak into public `spec`
   - treat dead public knobs with no live semantics as a real finding
   - treat nested provider scaffolding as a real finding when the same UX can
     stay close to user intent with a simpler public shape
   - for any artifact/publication change, ask explicitly:
     - what is the exact published source of truth?
     - what exact artifact or published state is canonical?
     - are concrete tool or backend brands leaking into public contract?
   - for any storage/data-plane change, ask explicitly:
     - what is the exact byte path end-to-end?
     - how many full-size copies may exist at once?
     - where do those copies live: object storage, PVC, `emptyDir`, temp dir?
     - is the path streaming or does it require local materialization?
     - what requests/limits/size limits protect the node?
   - for any "metadata/history/lineage" change, ask explicitly:
     - what exact fields are written?
     - who consumes them?
     - are they a source of truth or only audit/history?
     - do they duplicate public `status` or another backend state machine?
   - for any large-model claim, require a concrete worst-case resource answer
     instead of prose like "uses staging" or "publishes asynchronously"
8. If the task was substantial or used delegation, confirm whether a final `reviewer` pass is still required.
9. Return only concrete findings, missing checks, and residual risks.

## Output

A short final review that helps decide whether the change is safe to keep.
