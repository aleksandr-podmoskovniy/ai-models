# HF publication e2e validation

## 1. Заголовок

Проверить живой end-to-end путь публикации двух публичных моделей `Hugging Face`
разных форматов и зафиксировать реальный byte path, копии и отклонение от
целевой картины

## 2. Контекст

В кластере уже поднят свежий `ai-models-controller`, а текущий live baseline
для remote `source.url` работает в режиме `Direct`:

- пользователь создаёт `Model` с `spec.source.url`;
- controller вычисляет формат и метаданные автоматически;
- publication идёт в канонический OCI `ModelPack` во внутренний `DMCR`;
- тяжёлые layer blobs идут через `DMCR direct-upload`;
- текущий runtime baseline уже отделён от старого backend shell.

Пользователь просит не теоретический пересказ, а живую проверку пути на двух
реальных моделях разных форматов и детальный разбор:

- откуда и как идут байты;
- что и где копируется;
- что перепаковывается;
- каких это объёмов;
- насколько это совпадает с целевой картиной модуля.

## 3. Постановка задачи

Нужно выполнить bounded эксплуатационную проверку текущего publication/runtime
baseline:

1. Взять две публичные модели `Hugging Face` разных форматов:
   - одну `Safetensors`;
   - одну `GGUF`.
2. Прогнать их через текущий live publication path в кластере.
3. Зафиксировать реальный путь данных по шагам:
   - `Hugging Face`;
   - publication worker;
   - временные/промежуточные границы;
   - object source или mirror boundary;
   - `DMCR`;
   - итоговый опубликованный артефакт;
   - при наличии, workload/runtime materialization boundary.
4. Отдельно посчитать и описать:
   - какие полные копии существуют одновременно;
   - где именно они лежат;
   - какие объёмы проходят через каждый шаг;
   - где есть packaging/repackaging.
5. Сопоставить получившуюся фактическую картину с целевой архитектурой модуля.

## 4. Scope

- новый bundle `plans/active/hf-publication-e2e-validation/*`;
- live cluster objects для временной проверки:
  - временный namespace для smoke;
  - два временных `Model`;
- operational inspection через `kubectl`, controller/source-worker logs и
  object/registry metadata;
- при необходимости обновление:
  - `images/controller/TEST_EVIDENCE.ru.md`.

## 5. Non-goals

- не переключать cluster-wide `artifacts.sourceFetchMode` с текущего `Direct`
  на `Mirror`;
- не тестировать `spec.source.upload`;
- не тащить в эту задачу `vLLM`, `KubeRay` или workload benchmark;
- не менять public API `Model` / `ClusterModel`;
- не делать blanket cleanup чужих объектов в кластере.

## 6. Затрагиваемые области

- live publication/runtime path в кластере `k8s-main`;
- controller publication observability;
- `Hugging Face` remote ingest path;
- `DMCR` publication backend contract;
- operational evidence surfaces, если будет полезно закрепить результаты в
  репозитории.

## 7. Критерии приёмки

- Созданы и прогнаны два реальных `Model` из публичного `Hugging Face` с
  разными входными форматами: `Safetensors` и `GGUF`.
- Для каждого прогона зафиксированы:
  - исходный репозиторий и ревизия;
  - выбранные source files;
  - итоговый `status.artifact`;
  - итоговый `status.resolved.format`;
  - publication worker runtime evidence.
- Есть детальный byte-path разбор для текущего live режима `Direct`:
  - где поток streaming;
  - где создаётся полная копия;
  - где происходит упаковка в OCI `ModelPack`;
  - какие объёмы участвуют на каждом шаге.
- Есть отдельный вывод по количеству полных копий и по узким местам текущей
  схемы.
- Есть отдельное сравнение "как сейчас" против "какая целевая картина".
- После проверки временные объекты либо удалены, либо явно перечислены как
  оставленные намеренно.

## 8. Риски

- можно случайно выбрать `GGUF`-репозиторий с несколькими `.gguf` файлами и
  проверить не тот сценарий;
- живой publication path может упереться в внешнюю сеть `Hugging Face`, а не
  во внутреннюю логику модуля;
- без аккуратного съёма логов легко потерять реальные объёмы и перепутать
  streaming/object-source path с локальной materialization;
- если публикация не завершится, важно зафиксировать именно место сбоя, а не
  подменить результат рассуждением по коду.
