---
name: platform-runtime-integration
description: Use for managed platform runtime wiring: auth, ingress, TLS, storage, HA, observability, and global Deckhouse integration.
---

# Platform runtime integration

## Read first

1. `AGENTS.md`
2. `docs/development/TZ.ru.md`
3. `docs/development/PHASES.ru.md`
4. current `plans/active/<slug>/PLAN.ru.md`

## Use this skill when

- the task changes ingress, HTTPS, certificate handling, HA, auth, SSO, storage, or observability;
- the task wires a managed internal service into DKP runtime primitives;
- the task changes interaction between module settings and global platform settings.

## Workflow

1. Reuse Deckhouse platform mechanisms before inventing local knobs.
2. Keep auth, TLS, storage, and monitoring consistent with DKP contracts.
3. Make runtime prerequisites explicit.
4. Keep DKP user-facing RBAC coverage aligned with Deckhouse access levels and
   global-vs-local ownership.
5. Prefer stable platform behavior over service-specific convenience options.
6. Keep operational semantics understandable without reading the source code.

## Hard rules

- No local convenience option that bypasses a DKP global mechanism without a
  plan-level justification.
- No hidden auth/TLS/storage prerequisite.
- No module-local RBAC grant that widens Secret, exec, port-forward, or
  internal service access without explicit plan-level justification.
- No observability contract that exists only in logs or chat and not in repo
  docs/templates/alerts.

## Output

A managed runtime integrated into DKP in a predictable way.
