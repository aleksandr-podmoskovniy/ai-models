# Plan: Rewrite Archived SKALA GPUDirect Runbook

## 1. Current phase

Задача не меняет продуктовую архитектуру `ai-models` и не открывает новый
phase workstream. Это documentation continuation поверх уже завершённого
archived исследования. Текущий rewrite не меняет технический результат
workstream, но меняет framing документа: главным предметом должен стать
`GPUDirect RDMA`, а не общий обзор всех соседних тем.

## 2. Orchestration

`solo`

Причина:

- меняется один archived operator-facing документ и сопровождающий active
  bundle;
- архитектурной неопределённости нет, новый runtime/API behavior не
  проектируется;
- delegation здесь добавит шум, а не сигнал.

## 3. Slices

### Slice 1. Normalize the rewrite task bundle

Цель:
- оформить continuation bundle перед любыми содержательными правками.

Файлы:
- `plans/active/rewrite-skala-sdn-rdma-smoke/TASK.ru.md`
- `plans/active/rewrite-skala-sdn-rdma-smoke/PLAN.ru.md`

Проверки:
- bundle создан в `plans/active/` без конфликта slug;
- scope явно ограничен rewrite-only задачей.

Артефакт результата:
- компактный active bundle для текущего rewrite.

### Slice 2. Reframe the archived runbook structure

Цель:
- заново собрать линейную структуру archived `SKALA` документа вокруг
  основного сценария `GPUDirect RDMA`;
- провести полноценный редакторский review pass по языку, логике и
  последовательности повествования.

Файлы:
- `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:
- introduction и entry criteria делают `SKALA` самодостаточным документом и
  не отправляют читателя в другие runbook;
- документ ведёт читателя по последовательности
  `prerequisites на узле -> proof на узле -> prerequisites в pod ->
  inter-pod proof -> optional control measurements`;
- верх документа быстро отвечает на практические вопросы о стенде, VM
  prerequisites, пакетах и роли трёх тестовых pod;
- language pass убирает машинные формулировки, случайный jargon и избыточную
  детализацию без потери технической точности;
- technical baseline, measured values и command lines не дрейфуют.

Артефакт результата:
- цельный operator-facing runbook, в котором `GPUDirect RDMA` является
  центральной целью, а обычный `RDMA` и `TCP` служат контрольными проверками.

### Slice 3. Final consistency and review

Цель:
- проверить diff на scope drift, потерю важных caveat и лишние повторы.

Файлы:
- текущий bundle
- `SKALA-SDN-RDMA-SMOKE.ru.md`

Проверки:
- `git diff -- plans/active/rewrite-skala-sdn-rdma-smoke plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`
- structured final review по `review-gate`

Артефакт результата:
- готовый rewrite с зафиксированными residual risks, если они останутся.

## 4. Rollback point

Безопасная точка отката:

- после создания active bundle, но до переписывания archived `SKALA` файла.

На этом этапе в репозитории уже есть только planning surface для текущего
rewrite, но ещё нет риска нарушить канонический archived runbook.

## 5. Final validation

- визуально проверить целостность rewritten `SKALA` runbook;
- проверить, что верх документа не содержит навигационных ссылок на другие
  runbook;
- просмотреть итоговый diff только по затронутым файлам;
- выполнить `review-gate` как финальный structured docs review.
