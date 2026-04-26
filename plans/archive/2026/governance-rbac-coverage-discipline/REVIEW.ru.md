## Review gate — 2026-04-24

### Findings

Финальный reviewer нашёл один governance consistency gap:

- `runtime entrypoints` были указаны в `AGENTS.md` и
  `task-intake-and-slicing`, но не во всех нижних workflow surfaces.

Gap закрыт: downstream docs/agent profiles/review-gate теперь используют
одинаковый trigger `API/auth/RBAC/exposure/runtime entrypoint`.

Product RBAC plan также скорректирован по reviewer findings:

- RBAC grants больше не планируются до public-surface hardening;
- добавлен Deckhouse source revision;
- добавлена access-review matrix для allow/deny paths.

Follow-up reviewer нашёл последний blocking gap:

- access-review matrix не требовала явный deny для `pods/log`;
- PLAN Slice 3 не называл sensitive runtime paths в артефакте.

Gap закрыт: `ACCESS_REVIEW.ru.md` теперь требует deny для module-local
`pods/log`, а `PLAN.ru.md` требует cases для `pods/log`,
exec/attach/port-forward/proxy и internal runtime resources.

### Что проверено

- RBAC discipline добавлен в reusable core, а не в `ai-models`-specific
  overlays.
- Новый skill/agent не создан; tightened existing boundaries:
  `task-intake-and-slicing`, `review-gate`, `k8s-api-design`,
  `platform-runtime-integration`, `task_framer`, `api_designer`,
  `integration_architect`, `reviewer`.
- Workflow docs требуют RBAC coverage evidence для relevant
  API/auth/RBAC/exposure/runtime entrypoint tasks.
- `.codex/governance-inventory.json` синхронизирован с новыми required
  phrases.

### Проверки

- `python3 -m json.tool .codex/governance-inventory.json`
- `make lint-codex-governance`
- `git diff --check`

### Residual risks

- Product RBAC templates ещё не реализованы; они вынесены в отдельный bundle
  `plans/active/rbac-access-level-coverage/`.
- API hardening blockers не закрыты этим governance slice:
  cleanup-handle annotation leak, `ClusterModel` cross-namespace
  `authSecretRef`, public upload token reference, runtime-specific condition
  reasons.
