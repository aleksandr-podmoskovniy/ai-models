## 1. Заголовок

Упрочнение publication byte-path для больших моделей и явный storage UX

## 2. Контекст

Текущий phase-2 controller path публикует `Model` / `ClusterModel` через
controller-owned publish worker. Для remote `HuggingFace` source runtime уже
умеет держать durable source mirror в object storage под `.mirror`, а затем
materialize'ить snapshot и checkpoint в локальный workspace publish worker.

При этом live runtime по умолчанию запускается с:

- `publicationRuntime.workVolume.type=EmptyDir`;
- `publicationRuntime.workVolume.emptyDir.sizeLimit=50Gi`;
- `publication worker resources requests/limits.ephemeral-storage=50Gi`.

Это не RAM, а node ephemeral storage, но текущий platform UX почти не
объясняет этого оператору заранее. На smoke-прогонах это уже всплыло как
позднее открытие: модель может быть существенно больше default workspace и
пользователь узнаёт об этом только по факту runtime behavior.

Логи также показывают structural drift в naming:

- controller event всё ещё говорит `remote raw ingest`;
- при этом фактический raw provenance уже указывает на
  `s3://.../source-url/.mirror/...`, то есть на source mirror, а не на старый
  raw-stage object set.

Нужно привести runtime, warnings, docs и naming к defendable сценарию для
больших моделей, включая кейсы порядка `2Ti`.

## 3. Постановка задачи

Нужно проработать и реализовать нормальный platform scenario для больших
моделей в publication path:

- controller по умолчанию не должен молча полагаться на `EmptyDir 50Gi`; default
  runtime path должен использовать `PersistentVolumeClaim` на default
  StorageClass, если storageClassName явно не переопределён;
- user-facing contract должен заранее объяснять, что `EmptyDir` расходует node
  ephemeral storage и не подходит как default assumption для очень больших
  моделей;
- remote path должен перестать маскировать source mirror как `raw ingest`, если
  live durable source of truth уже другой;
- нужно проверить, остаются ли старые remote raw-stage file copies реально
  живыми; если нет, не надо дальше документировать их как live object-storage
  behavior.

## 4. Scope

- анализ и фиксация live publication byte-path для `HuggingFace` remote source;
- выбор явного operator-facing сценария для больших моделей:
  - fail-fast guardrails;
  - clear warning / event / status surface;
  - documented `PersistentVolumeClaim` default path;
- перевод runtime/templates/OpenAPI defaults на PVC-backed publication workspace;
- корректировка controller/runtime naming вокруг `raw` vs `source mirror`
  без premature rename всего object-storage subtree;
- синхронизация docs, values/OpenAPI и test evidence с выбранным сценарием.

## 5. Non-goals

- не проектировать новый public API вокруг inference/runtime delivery;
- не делать в этом slice полноценный zero-local-copy publish pipeline;
- не менять DMCR artifact contract;
- не добавлять speculative values или public knobs без live controller semantics;
- не делать в этом slice полный rename `raw/` subtree, если он всё ещё нужен как
  shared namespace для upload staging и source mirror prefixes.

## 6. Затрагиваемые области

- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `images/controller/internal/adapters/k8s/workloadpod/*`
- `openapi/values.yaml`
- `templates/controller/*`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- `publicationRuntime.workVolume.type` по умолчанию становится
  `PersistentVolumeClaim`, а generated PVC использует default StorageClass, если
  `storageClassName` пустой.
- Docs и OpenAPI больше не создают впечатление, что `EmptyDir 50Gi` —
  platform default для publication runtime.
- При недостаточном workspace пользователь получает явный platform-facing
  сигнал:
  - explainable status / condition / message;
  - audit/event reason, из которого понятно, что не хватает local workspace;
  - указание, какой runtime knob нужно изменить (`EmptyDir` size или PVC mode).
- Документация явно говорит, что `publicationRuntime.workVolume.type=EmptyDir`
  расходует именно node ephemeral storage, а не память, и что для больших
  моделей нужен PVC-backed scenario.
- Operator-facing naming больше не называет source-mirror path `raw ingest`,
  если фактический durable source of truth уже `.mirror`.
- Документация и audit messages больше не описывают старые remote raw-stage
  file copies как live default behavior, если они уже вытеснены source mirror
  path.
- `Model` и `ClusterModel` остаются семантически выровненными: одинаковые
  guardrails, одинаковый status UX.
- Добавлены узкие тесты на:
  - size/capacity guardrail;
  - naming/status projection;
  - remote source path without dead raw-stage assumption.

## 8. Риски

- можно сломать существующий happy path для небольших моделей, если guardrail
  посчитает capacity слишком грубо;
- удаление remote raw-stage path может задеть cleanup/provenance semantics, если
  там остался скрытый live consumer;
- переход к PVC-first messaging может оказаться неожиданным для существующих
  small-scale smoke сценариев;
- warning-only решение без hard guardrail не решит реальную эксплуатационную
  проблему и снова оставит surprise-failure на runtime.
