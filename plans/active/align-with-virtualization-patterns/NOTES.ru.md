# Audit notes: `ai-models` vs `virtualization`

Дата: `2026-04-16`

## Drift matrix

### Aligned

- `Chart.yaml` / `charts/` / `.helmignore` follow DKP module chart shell.
- `openapi/config-values.yaml` vs `openapi/values.yaml` keeps the same
  user-facing vs internal split that `virtualization` uses.
- executable runtime code stays under `images/*`, with `images/controller/`,
  `images/hooks/`, `images/dmcr/`, `images/backend/`.
- root `werf.yaml` stays module-oriented, centralizes `SOURCE_REPO`,
  `SOURCE_REPO_GIT`, `GOPROXY`, `DistroPackagesProxy`, and reuses `.werf/*`
  helpers instead of copy-paste per image.
- GitHub workflows already follow the repo-local DKP pattern:
  `build.yaml` + `deploy.yaml`, with repo-local `make` entrypoints.

### Align now

- `.werf/stages/bundle.yaml` did not include `monitoring/` in release payload.
  This drifted from production module pattern used in both `virtualization` and
  `gpu-control-plane`, where monitoring assets are part of the shipped module
  contract.

### Intentional differences

- `ai-models` keeps controller/runtime source directly under
  `images/controller/`, while `virtualization` also has source/artifact split
  through `src/` and wrapper images. This is intentional because `ai-models`
  repo rules explicitly require executable runtime code under `images/*`.
- `ai-models` does not copy `virtualization`'s exact `.werf/*.yaml` topology.
  The current `.werf/stages/*.yaml` split is still module-oriented and does not
  create patchwork by itself.
- `build/components/versions.yml` remains build-time metadata. Unlike
  `monitoring/`, it is not an install-time payload and does not need to travel
  in the bundle by default.

### Deferred / no action

- no direct evidence that `release-channel-version` must import `module.yaml`
  for `ai-models`; `virtualization` does this, but `gpu-control-plane` does not,
  so this is not treated as a stable cross-module production requirement.

## Controller/runtime continuation

### Aligned

- `images/controller/internal/bootstrap/bootstrap.go` already acts as a proper
  composition root around `ctrl.Manager`: scheme assembly, owner registration,
  probes, and metrics wiring are centralized there instead of leaking into
  random packages.
- manager/controller runtime defaults are now explicit in the composition root:
  bridged logger, `RecoverPanic`, `UsePriorityQueue`, and a production-grade
  `CacheSyncTimeout` are configured centrally instead of relying on implicit
  library defaults.
- owner controller boundaries remain production-grade and map cleanly to
  `virtualization`'s `pkg/controller/<owner>` pattern:
  `catalogstatus`, `catalogcleanup`, `workloaddelivery`.
- `workloaddelivery` keeps watch/index/setup logic close to the owner package,
  similar to `virtualization`'s controller-local watcher discipline and
  `workload-updater` setup shell.
- `catalogstatus` now uses a metadata-only secondary watch for Pods because its
  mapping path consumes only labels/annotations; this removes unnecessary full
  Pod cache pressure from the manager.
- `internal/monitoring/catalogmetrics` already follows the same production idea
  as `virtualization`'s `pkg/monitoring/metrics/*`: collector registration,
  object iteration, descriptors, and report/emission logic are separated from
  reconcile code.
- file-size hotspots checked in this continuation stay within the controller
  LOC guardrails that the repo documents for production hygiene; no fat-shell
  or god-adapter regression was confirmed.

### Intentional differences

- `ai-models` keeps module-local runtime code under `internal/` rather than
  `pkg/`; this is consistent with repo rules and does not weaken production
  boundaries.
- `ai-models` splits command/config/resource/bootstrap concerns across small
  files, while `virtualization` still keeps more of that logic in
  `cmd/virtualization-controller/main.go`. For this repo that is an
  improvement, not a drift requiring rollback.
- `ai-models` exposes explicit cross-owner seams in `application/`, `ports/`,
  `adapters/`, and `dataplane/`, whereas `virtualization` often keeps analogous
  logic controller-local under `pkg/controller/<owner>/internal`. Because
  publication/upload/materialization paths are genuinely shared across multiple
  owners here, the explicit seams are justified.
- `catalogmetrics` remains one package instead of a per-kind metrics tree. With
  only `Model` and `ClusterModel` as the public truth surface, splitting it now
  would be premature patchwork.
- there is no separate global watchers package like
  `pkg/controller/watchers`; current watch logic does not yet have enough
  cross-owner reuse to justify that boundary.

### Align now

- `bootstrap` previously relied on implicit controller-runtime defaults for
  controller logger wiring, panic recovery, queue mode, and cache sync timeout.
  This was weaker than the explicit production setup used in `virtualization`.
- `catalogstatus` previously watched full `Pod` objects even though the enqueue
  mapping used only metadata. That created avoidable cache/memory pressure.
- `images/controller/STRUCTURE.ru.md` had live doc drift:
  the package map lagged behind the actual tree (`publishop`, `modelpack/oci`)
  and did not state which controller/runtime differences from `virtualization`
  are intentional versus real drifts.

### Current verdict

- no large-scale controller/runtime rewiring was justified by the reference
  audit on `2026-04-16`;
- confirmed production drifts in this continuation were fixed with bounded
  hardening changes in `bootstrap` and `catalogstatus`, plus documentation
  realignment;
- cosmetic source reshaping toward `virtualization` without local justification
  is still rejected.
