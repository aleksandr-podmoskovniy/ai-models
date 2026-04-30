2026-04-27T20:01:39Z
- Historical live evidence before the names-only SharedDirect contract was
  moved to
  `plans/archive/2026/live-e2e-ha-validation-legacy-evidence/k8s-apiac-20260429-001154/`.
- This active bundle now keeps only the next executable validation plan and
  current runbooks. Historical artifacts are not an executable source of truth
  for workload delivery.
- Next live run must validate comma-separated `ai.deckhouse.io/model` /
  `ai.deckhouse.io/clustermodel`, stable `/data/modelcache/models/<model-name>`
  paths, SharedDirect readiness and blocked-no-fallback behavior.
