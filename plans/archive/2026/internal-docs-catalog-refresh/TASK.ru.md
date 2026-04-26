### 1. Заголовок
Актуализация internal ADR по каталогу моделей под live `ai-models`

### 2. Контекст
В `internal-docs/2026-03-18-ai-models-catalog.md` сохранился более ранний
продуктовый и API narrative, который уже расходится с live состоянием
репозитория. В коде и repo-docs закрепились:

- узкий public contract `Model` / `ClusterModel` с `spec.source`;
- controller-owned publication в OCI `ModelPack` через внутренний `DMCR`;
- cluster-level `artifacts.sourceFetchMode` (`Direct` / `Mirror`) для remote
  `source.url`;
- controller-owned `spec.source.upload` flow через upload-session и staging;
- current workload delivery baseline через `materialize-artifact`,
  managed bridge volume и shared PVC bridge;
- landed `nodeCache` slice и runtime observability.

Нужно привести внешний ADR к этой реальности, не ломая его текущую структуру и
стилистику повествования.

### 3. Постановка задачи
Переписать содержимое
`/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
так, чтобы документ:

- оставался в той же структуре и narrative style;
- описывал только актуальные public contracts, runtime modes и user stories;
- убрал legacy-поля, legacy-assumptions и backend-first drift;
- объяснял current CRD, publication flow, registry ingest modes,
  workload delivery path, statuses/conditions и user-facing usage patterns.

### 4. Scope
В задачу входит:

- инвентаризация live public contract из `api/`, `crds/`, `docs/`,
  `images/controller/`, `images/dmcr/`;
- обновление всех разделов ADR без изменения его структуры;
- актуализация user stories, use cases, examples и field-by-field explanations;
- явное выравнивание текста с текущими режимами загрузки в registry и
  выгрузки в workload;
- вычищение утверждений о полях и поведении, которых больше нет в коде.

### 5. Non-goals
В задачу не входит:

- менять репозиторный public API, CRD, values или runtime behavior;
- реорганизовывать структуру самого ADR;
- редактировать repo-local governance surfaces;
- писать новый отдельный архитектурный документ вместо актуализации
  существующего.

### 6. Затрагиваемые области
- `plans/active/internal-docs-catalog-refresh/`
- `/Users/myskat_90/flant/aleksandr-podmoskovniy/internal-docs/2026-03-18-ai-models-catalog.md`
- как source of truth для сверки:
  - `api/core/v1alpha1/*`
  - `crds/*`
  - `docs/CONFIGURATION.ru.md`
  - `docs/README.md`
  - `images/controller/README.md`
  - `images/controller/STRUCTURE.ru.md`
  - `images/controller/TEST_EVIDENCE.ru.md`
  - `images/dmcr/README.md`

### 7. Критерии приёмки
- Структура ADR сохранена: существующие разделы и порядок следования не
  меняются.
- Текст ADR больше не утверждает наличие полей `spec.modelType`,
  `spec.inputFormat`, `spec.runtimeHints`, `spec.usagePolicy`,
  `spec.launchPolicy`, `spec.optimization` в live CRD, а корректно описывает
  текущий contract через `spec.source` и `status.*`.
- Документ корректно описывает текущие publication modes:
  - `spec.source.url` только для Hugging Face HTTPS;
  - `artifacts.sourceFetchMode=Direct|Mirror`;
  - `spec.source.upload` через upload-session/staging;
  - публикацию в OCI `ModelPack` через внутренний `DMCR` direct-upload v2.
- Документ корректно описывает current workload delivery baseline:
  - аннотации `ai.deckhouse.io/model` / `ai.deckhouse.io/clustermodel`;
  - `materialize-artifact` bridge;
  - `AI_MODELS_MODEL_PATH`, `AI_MODELS_MODEL_DIGEST`,
    `AI_MODELS_MODEL_FAMILY`;
  - managed fallback volume и shared PVC bridge;
  - landed `nodeCache` substrate/runtime slice без ложных обещаний
    workload-facing shared-direct path.
- Примеры `Model` / `ClusterModel`, описания `status.phase`,
  `status.upload`, `status.artifact`, `status.resolved` и `conditions`
  выровнены с live кодом и не содержат drift.
- Документ убирает или явно переписывает legacy narrative про historical
  backend, старые policy-поля и неlanded API semantics.

### 8. Риски
- Из-за сохранения структуры документа часть секций привязана к legacy
  заголовкам; нужно аккуратно переписать их содержимое так, чтобы они не
  возвращали пользователя к несуществующему contract.
- Live runtime богаче по внутренним деталям, чем public API; важно не
  перегрузить ADR internal-only implementation trivia.
- В рабочем дереве есть несвязанные незакоммиченные изменения; нельзя
  трогать их или опираться на них как на уже принятый baseline без явной
  проверки.
