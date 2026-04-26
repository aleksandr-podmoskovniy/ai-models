## 1. Заголовок

Единый публичный прогресс локальной загрузки в `Model.status`

## 2. Контекст

Сейчас локальная загрузка для `source.upload` уже имеет controller-owned
upload-session runtime, но публичный прогресс в API размазан: в `Model.status`
нет отдельного поля прогресса, а внешний наблюдатель может видеть только фазу,
conditions и вложенный `status.upload`.

В проекте `virtualization` аналогичные user-driven ingest flows используют
единое top-level поле `status.progress` в процентном формате и отдельную
`kubectl`-колонку `Progress`. Для `ai-models` нужен такой же публичный UX
именно для локальной загрузки.

Дополнительно в текущем upload-session path уже есть каркас для
`expectedSizeBytes`, но он не доведён до live flow: размер не сохраняется в
session state, поэтому процент загрузки честно считать пока не из чего.

## 3. Постановка задачи

Нужно сделать единый публичный прогресс локальной загрузки по образцу
`virtualization`:

- в `Model.status` / `ClusterModel.status` появляется top-level поле
  `progress`;
- у `Model` / `ClusterModel` появляется printcolumn `Progress`;
- для `source.upload` controller проецирует процент локальной загрузки прямо в
  `status.progress`;
- процент считается из реального persisted upload-session state, а не из текста
  conditions;
- controller перед проекцией `status.progress` сам освежает persisted multipart
  state из staging, а не ждёт, что это сделает внешний client polling against
  upload gateway;
- upload-session dataplane принимает `sizeBytes` на этапе `probe`, сохраняет
  его в session state и дальше использует как знаменатель для вычисления
  процента;
- старый progress-through-message narrative не становится вторым источником
  истины.

## 4. Scope

- обновить публичный API `ModelStatus`;
- добавить `Progress` printcolumn в `Model` и `ClusterModel`;
- довести upload-session probe contract до сохранения `sizeBytes`;
- вычислять процент загрузки из `ExpectedSizeBytes` и `UploadedParts`;
- протянуть progress через upload-session handle и status projection;
- синхронизировать docs и тесты.

## 5. Non-goals

- не проектировать progress для `HuggingFace` source;
- не добавлять отдельные метрики, CRD history-поля или второй progress API;
- не менять `DMCR` publication progress;
- не вводить новый public spec knob;
- не переделывать end-to-end upload client в этом срезе.

## 6. Затрагиваемые области

- `plans/active/upload-progress-status/*`
- `api/core/v1alpha1/*`
- `api/README.md`
- `images/controller/internal/ports/publishop/*`
- `images/controller/internal/adapters/k8s/uploadsession/*`
- `images/controller/internal/adapters/k8s/uploadsessionstate/*`
- `images/controller/internal/dataplane/uploadsession/*`
- `images/controller/internal/application/publishobserve/*`
- `images/controller/internal/domain/publishstate/*`
- `images/controller/internal/controllers/catalogstatus/*`
- `images/controller/README.md`
- `images/controller/TEST_EVIDENCE.ru.md`

## 7. Критерии приёмки

- В `Model.status` и `ClusterModel.status` есть top-level поле `progress`.
- `kubectl get model` и `kubectl get clustermodel` получают колонку
  `Progress`.
- Для `source.upload` в фазах подготовки/ожидания загрузки `status.progress`
  публикуется как процентная строка в стиле `virtualization` (`0%`, `17%`,
  `100%`).
- Процент вычисляется из persisted session state:
  `expectedSizeBytes` и суммы `uploadedParts[].sizeBytes`.
- `sizeBytes` можно передать в upload-session `probe`, и этот размер
  сохраняется в session secret/state.
- `status.progress` не дублируется отдельным вложенным progress field в
  `status.upload`.
- Узкие тесты вокруг `uploadsessionstate`, `uploadsession`, `publishobserve`,
  `publishstate` и `catalogstatus` проходят.
- Документация прямо описывает top-level `status.progress` как единый публичный
  индикатор локальной загрузки.

## 8. Риски

- если не сохранить `sizeBytes` в session state, процент будет либо фальшивым,
  либо пустым;
- если progress считать не из persisted multipart state, controller получит
  второй источник истины и drift;
- если не синхронизировать printcolumn, docs и tests, получится очередной
  разъезд публичного контракта.
