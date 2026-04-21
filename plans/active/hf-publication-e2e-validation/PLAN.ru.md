## 1. Current phase

Этап 1 для live publication path и одновременно проверка того, насколько
текущий runtime baseline уже готов к целевой прод-картине без лишних
intermediate seams.

## 2. Orchestration

`solo`

Причина:

- задача в основном эксплуатационная и исследовательская, а не архитектурно
  развилочная;
- решение не требует нового API или layout fork;
- текущая execution policy не разрешает звать subagents без прямого запроса
  пользователя, поэтому findings фиксируются прямо в bundle и итоговом
  разборе.

## 3. Slices

### Slice 1. Зафиксировать bundle и выбрать корректные HF-модели

Цель:

- подобрать две публичные модели разных форматов, которые соответствуют
  текущим правилам отбора файлов;
- не попасть в ложный `GGUF`-сценарий с несколькими квантами/шардами.

Файлы/каталоги:

- `plans/active/hf-publication-e2e-validation/*`

Проверки:

- live `Hugging Face` metadata inspection;
- manual consistency review against current `sourceFetchMode=Direct`.

Артефакт результата:

- две конкретные модели и ожидаемый selected-files set.

### Slice 2. Прогнать обе модели через live cluster publication path

Цель:

- создать временный namespace;
- применить два `Model`;
- дождаться терминального результата и собрать status/log evidence.

Файлы/каталоги:

- live cluster objects only

Проверки:

- `kubectl apply/get/describe/logs`;
- `kubectl get models -o yaml/json`;
- inspection of source-worker Pods and related Secrets/Events if needed.

Артефакт результата:

- живой operational evidence по двум публикациям.

### Slice 3. Восстановить реальный byte path и объёмы

Цель:

- по live status, логам, объектам и known runtime wiring восстановить точный
  путь байтов end-to-end;
- явно посчитать полные копии, packaging и верхние оценки по объёмам.

Файлы/каталоги:

- live cluster evidence;
- `docs/CONFIGURATION.ru.md`;
- touched controller/runtime code only for reading.

Проверки:

- cross-check live evidence against code/docs;
- manual consistency review.

Артефакт результата:

- детальный фактический byte path по `Safetensors` и `GGUF`.

### Slice 4. Сопоставить с целевой картиной и при необходимости закрепить evidence

Цель:

- дать операторское заключение:
  - где текущее поведение совпадает с целью;
  - где ещё есть дрифт/узкие места;
  - насколько текущий путь уже можно считать defendable prod baseline.
- при полезности зафиксировать результаты в `TEST_EVIDENCE`.

Файлы/каталоги:

- `images/controller/TEST_EVIDENCE.ru.md` при необходимости

Проверки:

- `git diff --check` если docs touched;
- `make verify` если repo files touched.

Артефакт результата:

- понятный разбор "как сейчас" против "куда идём".

## 4. Rollback point

После Slice 3: живой прогон уже выполнен и evidence собран, но никакие
repo-local docs ещё не менялись.

## 5. Final validation

- если изменялись только cluster objects: manual cleanup verification;
- если менялись repo files:
  - `git diff --check`
  - `make verify`
  - `review-gate`
