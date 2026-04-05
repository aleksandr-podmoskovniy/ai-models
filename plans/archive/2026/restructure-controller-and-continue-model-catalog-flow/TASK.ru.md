# Restructure Controller And Continue Model Catalog Flow

## 1. Контекст

В репозитории уже есть:

- public API `Model` / `ClusterModel` с `spec.source` и rebased `status`;
- controller runtime shell с HA, metrics, leader election и module wiring;
- internal packages для `publication`, `managedbackend`, `runtimedelivery`,
  cleanup и первого live HF import path;
- delete cleanup path через finalizer и cleanup Job;
- phase-1 backend runtime entrypoint `ai-models-backend-hf-import`.

При этом текущий live publish slice собран как proof-of-path и не соответствует
той архитектуре controller-а, о которой договорились:

- в одном reconciler смешаны lifecycle объекта, execution worker, pod result
  handoff и status projection;
- live path жёстко привязан к `mlflow`;
- publish/import auth distribution не повторяет controller-owned паттерн из
  virtualization/DVCR;
- runtime delivery и backend execution boundaries заданы только частично.

Пользовательский target flow остаётся таким:

1. В `Model` / `ClusterModel` выбирается `source` (`HuggingFace`, `Upload`,
   `HTTP` по аналогии с virtualization).
2. Модуль сохраняет модель либо в object storage через internal backend path,
   либо в OCI distribution path, и пишет ссылку на published artifact.
3. Запускается расчёт technical profile по ADR и enrichment `status.resolved`.
4. Объект проходит публичный lifecycle до `Ready`.
5. Downstream runtime использует модель через local materialization path
   (`init` / sidecar / agent -> PVC/shared volume -> local path).
6. При удалении CR controller очищает managed artifact после нужных проверок.

## 2. Постановка задачи

Исправить структуру controller-а под production-oriented boundaries и
продвинуть implementation дальше без смены публичной идеи API:

- отделить lifecycle owner `Model` / `ClusterModel` от execution/publication
  worker path;
- ввести явный internal publication operation/result boundary вместо чтения
  business state из pod termination log;
- выровнять controller-owned auth/access handoff по смыслу с virtualization;
- подготовить controller к нескольким source types и нескольким artifact
  backends без протаскивания backend details в public status;
- сохранить уже работающий cluster runtime shell и delete cleanup ownership;
- реализовать следующий рабочий slice поверх новой структуры, а не поверх
  текущего monolithic reconciler.

## 3. Scope

- `images/controller/internal/app/*`
- `images/controller/internal/modelpublish/*`
- new internal packages for publication operation / result / auth distribution
- `images/controller/internal/hfimportjob/*`
- `images/controller/internal/managedbackend/*`
- `images/controller/internal/runtimedelivery/*`
- `images/controller/cmd/ai-models-controller/*`
- `templates/controller/*` if runtime/RBAC wiring changes
- `api/core/v1alpha1/*` only if public status/conditions need minimal sync
- design/task docs affected by new controller boundaries
- bundle under
  `plans/active/restructure-controller-and-continue-model-catalog-flow/*`

## 4. Non-goals

- Не реализовывать за один шаг весь runtime-side materialization agent для
  `KubeRay` / `vLLM` / `KServe`.
- Не завершать сейчас все source types end-to-end, если это ломает corrective
  re-architecture.
- Не делать сейчас полную security hardening story для digest verification,
  attestation и supply-chain.
- Не переделывать phase-1 backend deployment shape без прямой необходимости.
- Не выносить raw MLflow/workspace/run/backend entities в public API.

## 5. Затрагиваемые области

- `api/`
- `images/controller/`
- `templates/controller/`
- `docs/` и `plans/`

## 6. Критерии приёмки

- Lifecycle ownership `Model` / `ClusterModel` отделён от publication worker
  orchestration и delete cleanup ownership.
- Controller больше не использует pod termination message как основной durable
  business-state channel между worker и reconciler.
- Есть явный internal operation/result contract, через который можно добавлять
  `HuggingFace`, `Upload`, `HTTP` и разные backend publishers без переписывания
  public reconciler.
- HF-first live path остаётся рабочим после re-architecture.
- Публичный `status` по-прежнему держит только source/artifact/resolved/phase/
  conditions и не раскрывает internal backend plumbing.
- Runtime delivery contracts остаются ориентированы на local materialization,
  а доступ к artifact path формализован отдельно от основного runtime pod.
- Delete path продолжает удалять managed artifact через finalizer-owned cleanup
  flow.
- Пройдены узкие tests и repo-level проверки для изменённых областей.

## 7. Риски

- Легко сломать уже живой HF path во время corrective refactor.
- Слишком рано можно зацементировать internal operation API и потом трудно
  добавить `Upload` / `HTTP` / OCI publication.
- Неправильное разделение auth/access path приведёт к утечке секретов в public
  contract или к переиспользованию controller env в worker pods.
- Если смешать delete cleanup и publication orchestration, снова появится
  неявный owner delete lifecycle.
