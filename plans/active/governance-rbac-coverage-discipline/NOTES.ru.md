## Read-only governance findings

`repo_architect`:

- reusable planning/review surfaces не требуют явно называть DKP
  user-facing RBAC coverage;
- новый skill/agent не нужен: RBAC coverage already belongs to reusable DKP
  API/integration/review core;
- project-specific overlays (`model-catalog-api`,
  `ai-models-backend-platform`, `backend_integrator`) менять не нужно;
- governance inventory must be updated, otherwise the new rule is not
  machine-checkable.

## Decision

Tighten existing reusable surfaces:

- `task-intake-and-slicing` and `task_framer`: bundle must capture RBAC
  coverage when relevant;
- `k8s-api-design` and `api_designer`: resource-level access semantics;
- `platform-runtime-integration` and `integration_architect`: Deckhouse
  global-vs-local RBAC wiring and auth boundary;
- `review-gate` and `reviewer`: final evidence and deny paths.

Do not create new `rbac` skill or agent.
