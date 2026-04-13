# REVIEW

## Findings

1. High: `AGENTS.md`, `.codex/README.md`, skills and agent profiles did not
   previously enforce a single governance precedence chain. That allowed local
   contradictions and “incidental” workflow edits without a dedicated bundle.
   Fixed by adding explicit precedence and governance-task rules.
2. High: the repo lacked an explicit reusable engineering doctrine for
   boundary discipline, long-context resilience, and systematic testing. That
   made it too easy to rely on chat memory or oversized bundles instead of
   durable repo-local rules. Fixed in `AGENTS.md` and `.codex/README.md`.
3. High: `controller-runtime-implementation` had become a catch-all dump of
   controller, API, package-map, and test-policy rules. That made the skill
   harder to apply consistently and duplicated neighboring skills. Fixed by
   shrinking it back to controller-runtime ownership and pushing adjacent
   concerns back to the proper skills.
4. High: controller architecture rules previously budgeted only production
   files. `_test.go` files could still grow into the same monolith shape the
   skill claimed to forbid. Fixed by encoding test-file LOC discipline in the
   controller-architecture reference and skill.
5. Medium: agent profiles did not explicitly treat workflow-governance edits as
   first-class scope. That made it too easy for implementation-oriented roles
   to treat skill/agent edits as incidental wording. Fixed in `task_framer`,
   `repo_architect`, `module_implementer`, and `reviewer`.
6. Medium: several core skills had weak workflow sections but no hard rules.
   That made them portable only as prose, not as enforceable baseline. Fixed
   by adding explicit hard-rule sections to module shell, config, API,
   platform integration, backend integration, and 3p integration skills.
7. Medium: `docs/development/CODEX_WORKFLOW.ru.md`,
   `TASK_TEMPLATE.ru.md`, and `REVIEW_CHECKLIST.ru.md` lagged behind the
   tightened governance doctrine. That left a second workflow narrative outside
   the machine-checked instruction surface. Fixed by aligning those docs and
   extending the governance inventory to cover them.

## Residual risks

- The repo-local governance surface now has a lightweight machine-checkable
  guardrail, but it still verifies only the highest-signal invariants. If the
  surface grows much further, the next corrective slice should extend the
  inventory instead of falling back to more prose.
- The baseline is now substantially more reusable across DKP module repos, but
  it still carries `ai-models` overlays inside the same repository. If this
  pattern is copied to another module, the adopter still needs to prune the
  project-specific overlays instead of copying everything blindly.
