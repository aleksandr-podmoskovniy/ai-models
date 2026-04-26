# Rewrite Archived SKALA GPUDirect Runbook

## Контекст

В `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/` уже
лежит завершённый исследовательский workstream по host-side и pod-level
`RDMA` / `GPUDirect RDMA` на стенде `RED OS 8 + Mellanox RoCE v2 + V100`.

Внутри этого архива документ
`SKALA-SDN-RDMA-SMOKE.ru.md` уже является канонической короткой инструкцией,
на которую ссылается соседний active workstream
`research-kuberay-vllm-v100-gpudirect-inference`.

Проблема текущего состояния не в нехватке данных, а в форме:

- документ смешивает introduction, runbook, benchmark verdict и toolkit notes;
- часть важных условий повторяется в нескольких местах;
- граница между host bootstrap, pod smoke и comparison block читается хуже,
  чем должна в operator-facing документе;
- главный фокус документа размыт между общим `RDMA` и `GPUDirect RDMA`, хотя
  практическая цель этого runbook — доказать именно `GPUDirect RDMA`;
- по мере локальных правок стало видно, что документу нужен не только rewrite,
  но и полноценный редакторский review pass: лишняя детализация, неудачные
  формулировки и местами неочевидная последовательность снижают практическую
  пользу для читателя;
- в worktree уже есть начатая локальная правка этого документа, но rewrite
  ещё не доведён до цельной структуры.

Нужен отдельный continuation bundle, который перепишет archived runbook без
изменения технического baseline и без создания второго competing source of
truth.

## Постановка задачи

Полностью переписать
`plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`
как один понятный operator-facing runbook с главным фокусом на
`GPUDirect RDMA`:

- сохранить фактические команды, метрики и проверенный стенд;
- убрать лишний narrative noise и повторы;
- сделать последовательность чтения и выполнения линейной;
- переписать документ нормальным инженерным русским языком без машинных
  оборотов, ложной важности и случайной графомании;
- явно развести:
  - какие шаги являются обязательными prerequisites для `GPUDirect RDMA`;
  - где именно доказывается путь `GPU memory -> RDMA`;
  - где заканчивается host bootstrap;
  - где начинается pod-level validation;
  - что считать functional proof, а что fastest profile;
  - что относится к основной проверке `GPUDirect RDMA`, а что является только
    контрольным замером обычного `RDMA` или `TCP`;
  - какие задачи документ покрывает сам и какие в него не входят.

## Scope

- создать отдельный active task bundle для rewrite-only continuation;
- зафиксировать scope и plan этого rewrite в `plans/active/<slug>/`;
- перечитать текущий archived `SKALA` runbook и убрать из него зависимость от
  внешних runbook или навигационных ссылок;
- переписать `SKALA-SDN-RDMA-SMOKE.ru.md` целиком, не меняя фактический
  технический baseline;
- сохранить все practically important commands, thresholds, sample results и
  troubleshooting guidance;
- сделать `GPUDirect RDMA` центральной линией документа, а обычный `RDMA` и
  `TCP` оставить как вспомогательные контрольные проверки;
- улучшить структуру, терминологию и пояснения для operator-facing чтения.
- довести документ до состояния, в котором он сначала объясняет стенд и
  логику проверки, а потом уже даёт команды.

## Non-goals

- не менять measured values, если для этого нет уже зафиксированного evidence;
- не вводить новый runtime/API/cluster contract;
- не переписывать `NOTES.ru.md`, `TASK.ru.md` или `PLAN.ru.md` archived
  workstream как часть этого среза;
- не превращать rewrite в новый research pass по `sdn`, `DRA` или `KubeRay`;
- не создавать новый канонический runbook в другом пути вместо правки
  существующего archived документа.

## Затрагиваемые области

- `plans/active/rewrite-skala-sdn-rdma-smoke/TASK.ru.md`
- `plans/active/rewrite-skala-sdn-rdma-smoke/PLAN.ru.md`
- `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/SKALA-SDN-RDMA-SMOKE.ru.md`
- как reference-only context:
  - `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/NOTES.ru.md`
  - `plans/archive/2026/research-sdn-underlay-rdma-dra-gpu-placement/TASK.ru.md`

## Критерии приёмки

- существует отдельный active bundle для этого rewrite-only continuation;
- `SKALA-SDN-RDMA-SMOKE.ru.md` остаётся единственным каноническим коротким
  runbook по archived `RDMA` workstream;
- верх документа сразу формулирует, что основная цель — доказать рабочий
  `GPUDirect RDMA`, а не просто наличие общего `RDMA` baseline;
- в документе явно разделены:
  - цель проверки;
  - prerequisites на узле;
  - доказательство `GPUDirect RDMA` на узле;
  - pod contract для `GPUDirect RDMA`;
  - pod-level `GPUDirect RDMA`;
  - контрольные замеры обычного `RDMA` и `TCP`;
  - toolkit and troubleshooting;
- документ остаётся самодостаточным и не отправляет читателя в другие runbook
  ради понимания своего основного назначения;
- команды и ожидаемые результаты остаются воспроизводимыми и не противоречат
  уже зафиксированным данным текущего файла и archived notes;
- из текста убраны явные повторы и сбивчивая смена narrative level;
- документ читается как линейный operator-facing runbook, а не как смесь
  журнала расследования и инструкции;
- верх документа быстро отвечает на практические вопросы читателя:
  какой стенд, почему фигурируют именно эти узлы и поды, что установлено, где
  брать пакеты, где обязательные prerequisites для VM;
- по тексту нет случайных ролей, терминологической путаницы и формулировок,
  которые звучат как машинный перевод или внутренний черновик;
- верхние разделы написаны в нейтральном инженерном тоне, без разговорной
  навигации вида "открывай его" и без зависимости от других документов.

## Риски

- при полном rewrite легко потерять отдельные practically important caveats,
  особенно вокруг `nvidia-peermem`, `bindingMode: DPDK`, `GID index` и
  различия между functional proof и fastest profile;
- из-за уже существующих локальных правок можно случайно затереть полезные
  улучшения, если не перепроверить текущий diff перед финализацией;
- избыточное упрощение может размыть точные команды и expected outputs, а это
  ухудшит воспроизводимость runbook.
