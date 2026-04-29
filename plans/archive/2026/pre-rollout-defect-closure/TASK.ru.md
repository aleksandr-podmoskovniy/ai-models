# Pre-rollout defect closure

## 1. Заголовок

Закрыть оставшиеся code-level дефекты перед новой выкладкой ai-models.

## 2. Контекст

После последнего большого e2e baseline стал рабочим, но остались четыре
важных дефекта, которые не должны ехать в следующий rollout как известный
долг:

- `Gemma 4` с Hugging Face `pipeline_tag=any-to-any` может давать пустые
  endpoint/features, но слепая публикация `any-to-any` как public task опасна
  для будущего `ai-inference` scheduler.
- workload delivery корректно отказывает без cache mount, но создаёт шумные
  retry events.
- DMCR GC всё ещё хранит большой raw `registryOutput` и пропускает шумные
  non-JSON SDK checksum warnings.
- log field dictionary не полностью выровнен: duration, digest, artifact,
  source, request, repository, phase.

## 3. Постановка задачи

Закрыть эти дефекты без расширения публичного CRD и без временных обходных
путей:

- сделать полезный deterministic profile для `any-to-any` только при локальном
  checkpoint evidence; broad upstream tag должен оставаться hint-only;
- сделать workload-delivery failure/backoff UX спокойным и объяснимым;
- убрать raw success-path GC output и подавить шумные SDK warnings;
- закрепить единый словарь ключевых structured log fields.

## 4. Scope

- `images/controller/internal/adapters/modelprofile/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/controllers/workloaddelivery/*`
- `images/controller/internal/adapters/k8s/modeldelivery/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/dmcr/internal/garbagecollection/*`
- `images/dmcr/internal/logging/*`
- controller runtime logging helpers where needed

## 5. Non-goals

- Не менять публичный CRD shape.
- Не добавлять новые access-level RBAC fragments.
- Не запускать live destructive e2e в этом slice.
- Не реализовывать ai-inference scheduler.
- Не переписывать все исторические logs вне затронутых runtime paths.

## 6. Затрагиваемые области

- Metadata/profile inference for catalog status.
- Workload delivery reconciliation and event emission.
- DMCR GC result persistence and logs.
- Shared structured logging conventions.

## 7. Критерии приёмки

- `any-to-any` даёт conservative endpoint/features только для multimodal class
  models с локальным checkpoint evidence, а без evidence деградирует в
  derived text/chat profile или hint-only path.
- Missing cache mount produces one stable warning/condition-style signal and
  bounded requeue/backoff, not event spam.
- Successful DMCR GC does not persist huge raw `registryOutput`; failure path
  keeps bounded debug evidence.
- Known checksum warning noise is not emitted as non-JSON operator log spam.
- New/changed logs use stable fields: `duration_ms`, `artifactDigest`,
  `artifactURI`, `sourceType`, `request`, `repository`, `phase`, `err`.
- Successful source-worker Pods and direct-upload state do not remain as
  unbounded runtime garbage after a `Model` / `ClusterModel` reaches `Ready`;
  cleanup-state remains the durable delete/finalizer source of truth.
- Targeted tests pass for changed packages; `git diff --check` passes.

## 8. Риски

- `any-to-any` is broad; the fix must be conservative and must not overclaim
  serving readiness.
- Event suppression must not hide real drift or recovery events.
- Removing raw GC output must keep enough failure evidence for debugging.
