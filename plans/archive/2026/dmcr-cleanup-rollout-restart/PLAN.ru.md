## 1. Slices

### Slice 1. Live evidence

- Inspect current `dmcr` Deployment, ReplicaSets and events.
- Compare old/new ReplicaSet PodTemplate annotations and specs.
- Inspect cleanup/GC related logs around rollout time.

Checks:

- `kubectl -n d8-ai-models get deploy,rs,pods`;
- `kubectl -n d8-ai-models get events --sort-by=.lastTimestamp`;
- `kubectl -n d8-ai-models get rs -l ... -o yaml`.

### Slice 2. Render/root-cause inspection

- Inspect `templates/dmcr/*`, hooks that sync Secrets/certs, and any checksum annotations.
- Determine whether Helm `lookup`/generated certs/secrets/config includes volatile data.

Checks:

- `rg checksum templates/dmcr templates hooks`;
- `make helm-template` if a template fix is made.

### Slice 3. Narrow fix

- Patch only the unstable template/code path.
- Do not alter public API or human RBAC.

Checks:

- `make helm-template`;
- `make kubeconform`;
- focused tests if Go code changes;
- `git diff --check`.

## 2. Notes

Live evidence starts at `2026-04-24 19:34:17 MSK`.

- `dmcr` Pods не crash-loop: Kubernetes создаёт новые ReplicaSet ревизии.
- Между rev53/rev54/rev55 менялись оба PodTemplate checksum:
  - `checksum/config` менялся при включении/выключении
    `aiModels.internal.dmcr.garbageCollectionModeEnabled`;
  - `checksum/secret` менялся даже при одинаковом config, потому что
    Deployment хешировал весь `templates/dmcr/secret.yaml`, а там есть
    `randAlphaNum`, `genCA`, `genSignedCert` и bcrypt `htpasswd`.
- `catalogcleanup` сейчас создаёт delete-triggered request сразу с
  `ai.deckhouse.io/dmcr-gc-switch`, поэтому hook немедленно включает
  maintenance/read-only mode и провоцирует rollout `dmcr`.
- Текущий scheduled request в кластере queued, без switch annotation:
  `dmcr-gc-scheduled`, `requested-at=2026-04-24T16:40:00.005574045Z`.
- Этот queued request удалён как live mitigation до выкатки исправленной
  версии, чтобы старая версия `dmcr-cleaner` не arm-ила очередной
  maintenance rollout после debounce.
