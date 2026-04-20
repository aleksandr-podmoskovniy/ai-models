## 1. Заголовок

Свести `Model` / `ClusterModel` public `spec` к source-only contract

## 2. Контекст

Текущий phase-2 public contract перегружен полями, которые пользователь не
должен руками рассчитывать:

- `spec.inputFormat`
- `spec.runtimeHints`
- `spec.modelType`
- `spec.usagePolicy`
- `spec.launchPolicy`
- `spec.optimization`
- `spec.displayName`
- `spec.description`

Live controller уже умеет сам вычислять значительную часть model metadata из
реального содержимого checkpoint'а и source-specific hints. При этом текущий
spec заставляет пользователя разбираться в формате модели, task,
model-type/policy semantics и launch hints, хотя это уже controller-owned
metadata path.

Пользовательский запрос жёсткий и оправданный: оставить в public `spec` только
то, что действительно выражает desired source of truth, а всё вычислимое
перенести в calculated metadata/status.

## 3. Постановка задачи

Нужно упростить public API `Model` / `ClusterModel` до source-only contract.

Целевой public `spec`:

```yaml
spec:
  source:
    url: https://huggingface.co/... | null
    authSecretRef:
      namespace: ""
      name: ""
    upload: {}
```

Где:

- `source.url` остаётся для remote source;
- `source.upload: {}` остаётся как явный discriminator upload-session path;
- `source.authSecretRef` остаётся только для private/gated remote source и не
  используется для upload path;
- всё остальное либо вычисляется controller'ом, либо публикуется только в
  `status.resolved`.

При этом нужно:

- убрать spec-driven policy validation;
- перестать требовать от пользователя `inputFormat` и `task`;
- выровнять upload path так, чтобы он жил без declared format / expected size
  contract из public `spec`;
- сохранить `Model` и `ClusterModel` семантически одинаковыми.

## 4. Scope

- упростить `api/core/v1alpha1` до source-only `spec`;
- перегенерить CRD/codegen;
- убрать из controller/runtime зависимость от spec-driven metadata/policy
  полей;
- упростить upload-session contract, где declared format и expected size больше
  не приходят из public `spec`;
- перенести model metadata intent в calculated `status.resolved`;
- синхронизировать docs и evidence с новым minimal contract.

## 5. Non-goals

- не проектировать новый inference API;
- не добавлять новый public metadata block вместо удалённых полей;
- не расширять source provider tree beyond current `source.url` / `source.upload`;
- не делать в этом slice full redesign of all status fields, если они уже
  описывают calculated metadata defendably;
- не менять publish byte-path, storage или DMCR contract.

## 6. Затрагиваемые области

- `api/core/v1alpha1/*`
- `api/core/*`
- `api/scripts/*`
- `crds/*`
- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/ingestadmission/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/adapters/modelprofile/*`
- `images/controller/internal/support/testkit/*`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`

## 7. Критерии приёмки

- Public `ModelSpec` / `ClusterModelSpec` больше не содержат:
  - `displayName`
  - `description`
  - `modelType`
  - `inputFormat`
  - `runtimeHints`
  - `usagePolicy`
  - `launchPolicy`
  - `optimization`
- В public `spec` остаётся только `source`:
  - `source.url`
  - `source.authSecretRef`
  - `source.upload`
- `source.upload` не требует дополнительных public child fields для happy path.
- `source.authSecretRef` остаётся разрешённым только для remote URL source.
- Controller больше не блокирует upload source из-за отсутствия
  `spec.runtimeHints.task`.
- Upload path больше не зависит от `spec.inputFormat` и
  `spec.source.upload.expectedSizeBytes`.
- Remote path больше не зависит от `spec.inputFormat`; input format
  определяется автоматически по remote files.
- Calculated metadata о формате, task, family, endpoints, runtimes и launch
  hints остаётся только в `status.resolved`.
- `status.conditions` больше не несут spec-policy mismatch reasons, которые
  опирались на удалённые public knobs.
- Generated CRD, codegen verify scripts, docs и examples согласованы с новым
  source-only contract.

### Architecture acceptance criteria

- public desired state описывает только источник модели, а не controller-owned
  publication metadata;
- computed model metadata не утекла обратно в новый spec под другими именами;
- upload-session и publish-worker runtime не получили новый скрытый spec-proxy
  contract вместо удалённых полей;
- `Model` и `ClusterModel` остаются одним и тем же semantic contract с разным
  scope, без divergence between namespaced and cluster behavior.

## 8. Риски

- для upload `GGUF` path может не остаться честного task inference, если
  текущий resolver всё ещё требует explicit task;
- можно сломать upload-session flow, если удалить declared format / expected
  size из spec, но не выровнять secret/runtime payloads;
- можно оставить hybrid docs/CRD/tests, где часть surface уже source-only, а
  часть всё ещё ждёт старые поля;
- можно неявно потерять полезную validation semantics, если убрать spec knobs
  без замены на defendable calculated metadata.
