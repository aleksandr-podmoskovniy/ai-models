## 1. Current phase

Этап 1: corrective continuation вокруг publication/runtime baseline.

Задача остаётся внутри внутреннего publication backend: исправляем прямую
загрузку больших слоёв в `DMCR`, не тащим сюда `DMZ`, workload delivery и
node-local cache.

## 1a. Execution status

Уже сделано в предыдущих срезах workstream:

- прямой multipart upload в backing storage вместо толстого registry `PATCH`
  path;
- `.dmcr-sealed` metadata/link, чтобы не делать полный `CopyObject` после
  multipart completion;
- controller-side checkpoint/resume для текущего direct-upload flow;
- basic progress/status projection вокруг publication runtime.

Долг этого continuation закрывается текущим change set:

- complete path больше не доверяет digest, присланному контроллером;
- digest/size вычисляются внутри `DMCR` по физическому объекту после multipart
  completion;
- второй полной копии объекта не появляется, остаётся один проверочный проход
  чтения.

## 1b. Implementation result 2026-04-23

Выполнено:

- active bundle переписан под целевую схему “прямая загрузка в хранилище +
  один проверочный проход чтения + no second full copy”;
- `DMCR direct-upload complete` возвращает `digest` и `sizeBytes`,
  вычисленные `DMCR`;
- `digest` в complete request стал optional expected digest;
- expected digest mismatch отклоняется, physical upload object удаляется;
- temporary verification read failure не удаляет physical upload object;
- повторный complete может продолжить проверку, если physical upload object
  уже был собран предыдущей попыткой;
- controller raw path строит descriptor из complete response;
- controller described path сверяет complete response с ожидаемым descriptor;
- тестовый direct-upload helper считает digest по фактическим uploaded bytes;
- docs/README синхронизированы с новым byte path.

Проверки:

- `cd images/dmcr && go test ./internal/directupload`
- `cd images/controller && go test ./internal/adapters/modelpack/oci/...`
- `make fmt`
- `make test`
- `make verify`

Результат:

- все проверки прошли локально;
- cluster rollout/validation не выполнялись.

## 2. Orchestration

`solo`

Причина:

- задача меняет внутренний `DMCR`/controller contract и документацию;
- по repo rules для такой задачи обычно нужен read-only review;
- в текущей сессии запуск сабагентов разрешён только по прямому запросу
  пользователя, поэтому работаем без delegation и фиксируем решения прямо в
  bundle.

## 3. Slices

### Slice 1. Зафиксировать целевую картину

Цель:

- закрепить схему “direct-to-S3 + один проверочный read + no second full copy”.

Файлы/каталоги:

- `plans/active/dmcr-zero-trust-ingest/TASK.ru.md`
- `plans/active/dmcr-zero-trust-ingest/PLAN.ru.md`

Проверки:

- ручная сверка с текущим stage-1 scope.

Артефакт результата:

- active bundle больше не обещает unsafe trusted digest path.

### Slice 2. Перевести `DMCR complete` на проверяемый digest

Цель:

- `DMCR` сам считает digest/size собранного physical object и возвращает их в
  ответе complete.

Файлы/каталоги:

- `images/dmcr/internal/directupload/*`

Проверки:

- `cd images/dmcr && go test ./internal/directupload`

Артефакт результата:

- missing digest allowed;
- expected digest mismatch rejected;
- successful complete writes repository link and `.dmcr-sealed` by
  `DMCR`-computed digest;
- no full object copy, only verification read.

### Slice 3. Перевести controller direct-upload client на digest из `DMCR`

Цель:

- raw path использует digest из complete response;
- described path сверяет returned digest/size с ожидаемым descriptor.

Файлы/каталоги:

- `images/controller/internal/adapters/modelpack/oci/*`

Проверки:

- `cd images/controller && go test ./internal/adapters/modelpack/oci/...`

Артефакт результата:

- fake helper в тестах считает digest по фактическим uploaded bytes;
- manifest/checkpoint получают digest из `DMCR`-owned complete result.

### Slice 4. Синхронизировать документацию

Цель:

- убрать stale narrative “controller digest is trusted” и честно описать
  byte path для больших моделей.

Файлы/каталоги:

- `docs/CONFIGURATION.ru.md`
- `docs/CONFIGURATION.md`
- `images/dmcr/README.md`
- при необходимости notes предыдущего active bundle

Проверки:

- `rg -n "trusted digest|controller-provided|no longer re-reads|не перечитывает|перечитывает полностью" docs images/dmcr plans/active`

Артефакт результата:

- docs говорят то же самое, что делает код.

### Slice 5. Финальная проверка

Цель:

- убедиться, что изменение поддерживаемо и не ломает общий модуль.

Проверки:

- `make fmt`
- `make test`
- `make verify`

Артефакт результата:

- задача готова к пользовательскому push/rollout без локального обращения к
  кластеру.

## 4. Rollback point

Безопасный rollback после Slice 1: только документы плана изменены, runtime
code ещё не тронут.

После Slice 2 rollback делается как единый откат изменений в
`images/dmcr/internal/directupload/*`, потому что request/response contract
ещё не должен расходиться с controller-side client.

## 5. Final validation

- `cd images/dmcr && go test ./internal/directupload`
- `cd images/controller && go test ./internal/adapters/modelpack/oci/...`
- `make fmt`
- `make test`
- `make verify`
