## Live acceptance 2026-04-25

Evidence dir: `/tmp/ai-models-e2e-20260425-050819`.

### Версия и baseline

- `Module/ai-models`: `Ready`, `LastReleaseDeployed=True`, `IsOverridden=True`,
  version `main`.
- Live images:
  - controller: `ghcr.io/aleksandr-podmoskovniy/modules/ai-models@sha256:72adce28884be7be245720d01bb244fa17233a0a28132e48d9610a9dc20a2fd3`;
  - upload/runtime image: `...@sha256:05d99d2d64f635e22f61abc49b2165f8a109f2333bf1d3fe0ebbc33fe71ee541`;
  - DMCR image: `...@sha256:9bf65c6b4afebab49c556d9fce2f49ad33379fb5ab8d9783f812baf1fecee7ef`.
- Baseline и final restart counters для `controller`, `upload-gateway`,
  `dmcr`, `dmcr-direct-upload`, `dmcr-garbage-collection`,
  `kube-rbac-proxy`: `0`.

### Publication и delivery

- Тестовая модель `ai-models-smoke/e2e-phi-20260425-050819` опубликована за
  ~14 секунд.
- Status:
  - `phase: Ready`;
  - digest `sha256:430809d43231b77dd8b4e64ec098976d77b6081838f23dd2328adbc36fbd6526`;
  - size `382119`;
  - format `Safetensors`;
  - family `phi`.
- DMCR direct-upload logs содержат layer size, declared digest, verification
  policy/source, fallback reason `checksum-missing`, final digest и duration.
- Delivery rollout на workload
  `Deployment/e2e-phi-20260425-050819-consumer` успешен.
- Materializer logs полные: remote inspect, layer extraction, destination
  replace, marker write, final completion.
- В workload проверены materialized files, marker JSON и env:
  `AI_MODELS_MODEL_DIGEST`, `AI_MODELS_MODEL_FAMILY`,
  `AI_MODELS_MODEL_PATH`.

### Delete, cleanup и GC

- Consumer deployment, ready model и failed publish test model удалены.
- Successful cleanup job:
  `ai-model-cleanup-3749bb32-40a8-4ff8-a8b7-7987f161a00a`, `Complete`, `1/1`,
  container restart `0`.
- Cleanup logs полные:
  - `artifact cleanup started`;
  - `artifact cleanup completed`;
  - `handle_kind=BackendArtifact`;
  - `dry_run=false`.
- Failed publish test model не создал cleanup job за 180 секунд, потому что
  cleanup state для backend artifact не был создан. Runtime state secret был
  удалён, namespace очищен.
- DMCR GC request после cleanup ожидаемо находился в queued-состоянии около
  10 минут (`activation_delay=10m`), затем получил `dmcr-gc-switch`,
  выполнил GC и был удалён.
- GC logs подтверждают maintenance gate quorum `2/2`, request activation,
  deletion of four blobs and request removal.
- Финальное состояние:
  - в `ai-models-smoke` остались только постоянные smoke secrets;
  - в `d8-ai-models` нет `dmcr-gc-request` secrets;
  - DMCR/controller pods running, restarts `0`.

### RBAC

- Live `user-authz` ClusterRoles присутствуют для `User`,
  `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`, `ClusterAdmin` с
  annotation `user-authz.deckhouse.io/access-level`.
- Live `rbacv2/use` и `rbacv2/manage` ClusterRoles присутствуют с aggregate
  labels `rbac.deckhouse.io/aggregate-to-kubernetes-as`.
- Exact-role `kubectl auth can-i` через временные ServiceAccount/Binding
  подтвердил:
  - `User`/`PrivilegedUser`: read-only `models` и `clustermodels`, без create/delete;
  - `Editor`/`Admin`: create/update/delete только namespaced `models`,
    `clustermodels` read-only;
  - `ClusterEditor`/`ClusterAdmin`: create/update/delete `models` и
    `clustermodels`;
  - module-local deny для `secrets`, `pods/log`, `pods/exec`, `pods/attach`,
    `pods/portforward`, `models/status`, `models/finalizers`,
    `jobs.batch` в `d8-ai-models`.
- `rbacv2/use` нельзя напрямую подвязать через `ClusterRoleBinding`:
  Deckhouse webhook требует `RoleBinding`. Через namespaced `RoleBinding`
  `use:view` даёт read-only `models`, `use:edit` даёт edit `models`, без
  cluster-scoped `clustermodels` и без internal/runtime доступа.

### Осталось доработать

1. Publication должен retry/requeue transient DMCR `503 UNAVAILABLE` при
   read-only GC maintenance window. Сейчас second publish завершился
   terminal `Failed` при коротком GC окне.
2. Source/publish worker log retention недостаточен для короткоживущих Pods:
   первый успешный publish pod исчез до снятия логов; watcher должен либо
   retry `ContainerCreating`, либо сохранять worker logs/event snapshots в
   controller-owned evidence/status/audit.
3. Delete path имеет встроенную задержку GC минимум `10m`. Это корректно по
   текущему DMCR design, но для UX/status нужно явно показывать queued/armed
   GC state, иначе оператор видит "хвост" `dmcr-gc-*` как возможную утечку.
4. `rbacv2/use` coverage для cluster-scoped `ClusterModel` через обычный
   namespaced `RoleBinding` не работает. Нужно сверить с Deckhouse intended
   aggregation path и явно зафиксировать, должен ли `use` управлять
   `ClusterModel` или это только `manage`/cluster personas.
