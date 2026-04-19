## 1. Заголовок

Удаление устаревшего publication work-volume shell и выравнивание streaming storage contract

## 2. Контекст

Текущий phase-2 controller path публикует `Model` / `ClusterModel` через
controller-owned publish worker. За последние streaming slices live runtime
уже ушёл от успешного `checkpointDir`/workspace path:

- direct `HuggingFace` publish идёт как remote object-source;
- mirrored `HuggingFace` publish идёт как source-mirror object-source;
- staged upload publish идёт как archive/object-source fast paths;
- успешного local materialization fallback в publish-worker shell больше нет.

При этом live runtime по умолчанию запускается с:

- `publicationRuntime.workVolume.type=PersistentVolumeClaim`;
- generated `PersistentVolumeClaim` `ai-models-publication-work`;
- default PVC size `50Gi`;
- `publication worker resources requests/limits.ephemeral-storage=50Gi`.

Это больше не отражает live byte path. По коду publish-worker уже не создаёт
workspace, `--snapshot-dir` удалён, а `ensureWorkspace(...)` больше не
используется в runtime. Значит `50Gi` PVC и связанный work-volume contract
стали stale pod shell, а не defendable storage requirement.

Логи также показывают structural drift в naming:

- controller event всё ещё говорит `remote raw ingest`;
- при этом фактический raw provenance уже указывает на
  `s3://.../source-url/.mirror/...`, то есть на source mirror, а не на старый
  raw-stage object set.

Нужно привести runtime, warnings, docs и naming к defendable сценарию для
больших моделей, включая кейсы порядка `2Ti`.

## 3. Постановка задачи

Нужно довести storage contract до текущей streaming-семантики:

- убрать publication worker work-volume/PVC shell, если live runtime больше не
  имеет локального workspace;
- убрать misleading default `50Gi` publication PVC из templates/OpenAPI/docs;
- оставить и задокументировать только реально живой bounded local storage
  contract publish-worker pod: `ephemeral-storage` для logs/writable layer;
- выровнять operator-facing docs и architecture surfaces под текущий byte path,
  где успех больше не требует local materialization.

## 4. Scope

- анализ и фиксация live publication byte-path для publish-worker;
- удаление stale publication work-volume/PVC shell из runtime/templates/OpenAPI;
- выравнивание sourceworker runtime options под live streaming path без
  мёртвого shared `workloadpod` boundary;
- синхронизация docs, values/OpenAPI и test evidence с новым storage contract.

## 5. Non-goals

- не проектировать новый public API вокруг inference/runtime delivery;
- не менять DMCR artifact contract;
- не делать в этом slice новый node-local cache/runtime delivery дизайн;
- не менять upload-gateway или workload-delivery cache contract.

## 6. Затрагиваемые области

- `images/controller/internal/application/publishplan/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/dataplane/publishworker/*`
- `images/controller/internal/adapters/sourcefetch/*`
- `images/controller/internal/adapters/k8s/sourceworker/*`
- `openapi/values.yaml`
- `templates/controller/*`
- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- sourceworker Pod больше не монтирует publication work volume и не получает
  `TMPDIR=/var/lib/ai-models/work`;
- `publicationRuntime.workVolume.*` и generated
  `PersistentVolumeClaim ai-models-publication-work` больше не входят в live
  runtime/template contract;
- `publication worker` resources по умолчанию больше не резервируют legacy
  `50Gi` local workspace budget; остаётся только bounded `ephemeral-storage`
  contract для writable layer/logs;
- docs и OpenAPI больше не описывают publication worker как PVC-backed
  workspace runtime;
- `images/controller/README.md`, `STRUCTURE.ru.md` и `TEST_EVIDENCE.ru.md`
  больше не документируют retired `workloadpod` boundary как live publish path;
- добавлены узкие тесты на:
  - sourceworker pod без work volume/TMPDIR shell;
  - controller runtime config без publication work-volume flags;
  - render output без publication work PVC.

## 8. Риски

- можно сломать sourceworker pod rendering, если старый shared work-volume
  contract всё ещё неявно нужен какому-то runtime branch;
- можно недооценить реальный worst-case writable-layer budget и слишком сильно
  занизить default `ephemeral-storage`;
- docs bundle вокруг publication storage уже успел описать PVC-first scenario,
  поэтому нужно аккуратно зачистить устаревший operator-facing narrative.
