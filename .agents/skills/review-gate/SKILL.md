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
6. If controller code changed, check whether controller quality gates and corrective architecture rules were respected.
7. If the task was substantial or used delegation, confirm whether a final `reviewer` pass is still required.
8. Return only concrete findings, missing checks, and residual risks.

## Output

A short final review that helps decide whether the change is safe to keep.
