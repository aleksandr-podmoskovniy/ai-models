# Phase-2 Runtime Follow-ups

## Контекст

Предыдущий giant bundle по corrective rebase phase-2 runtime накопил слишком
много закрытой истории и перестал быть удобной рабочей поверхностью.

Он архивирован в:

- `plans/archive/2026/rebase-phase2-publication-runtime-to-go-dmcr-modelpack/`

Текущий baseline после архивирования:

- phase-2 runtime controller-owned и Go-first;
- canonical published artifact:
  - internal `DMCR`
  - immutable OCI `ModelPack`
- current public source contract:
  - `HuggingFace`
  - `Upload`
- controller structure и test-discipline уже выровнены:
  - package map зафиксирован в `images/controller/STRUCTURE.ru.md`
  - coverage inventory зафиксирован в `images/controller/TEST_EVIDENCE.ru.md`
  - test-file LOC gate уже landed.

Нужен новый компактный canonical active bundle, чтобы следующие bounded slices
шли без повторного разрастания `plans/active`.

## Постановка задачи

Перезапустить phase-2 runtime planning surface:

- убрать giant historical bundle из `plans/active`;
- оставить его как инженерную историю в `plans/archive/2026`;
- создать новый короткий active bundle с текущим baseline, открытыми
  workstreams и validation expectations;
- зафиксировать planning hygiene, чтобы следующие active bundles снова не
  превращались в многотысячную летопись.

## Scope

- архивировать старый active bundle;
- создать новый canonical active bundle для следующих phase-2 runtime slices;
- перенести в него только:
  - current baseline
  - active invariants
  - current open workstreams
  - validation rules
- обновить planning hygiene docs, если это нужно для закрепления правила.

## Non-goals

- не менять runtime code, API, values, templates или текущий product scope;
- не переписывать архивированный bundle задним числом;
- не дробить историю на несколько новых active bundles одновременно;
- не переносить в новый bundle весь старый review log и slice-by-slice history.

## Затрагиваемые области

- `plans/active/*`
- `plans/archive/2026/*`
- `plans/README.md`

## Критерии приёмки

- giant bundle больше не лежит в `plans/active`;
- в `plans/active` остаётся один компактный canonical bundle для phase-2
  runtime continuation;
- новый bundle явно ссылается на archived predecessor как на историю, а не
  дублирует её;
- planning hygiene rule про oversized active bundles зафиксирован в repo docs;
- layout `plans/active` и `plans/archive` остаётся чистым и понятным.

## Риски

- можно потерять текущий baseline, если новый bundle будет слишком пустым;
- можно оставить параллельные active bundles и снова получить split-brain;
- можно случайно дублировать в новом bundle старую историю вместо новой
  компактной рабочей поверхности.
