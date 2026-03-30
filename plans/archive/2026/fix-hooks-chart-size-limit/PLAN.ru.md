# План: починить oversized go hooks chart artifact

## Current phase
Этап 1. Managed backend inside the module.

## Режим orchestration
`solo`.
Причина: основной риск уже локализован в hooks packaging/build shell; для этой задачи быстрее и чище провести прямую диагностику и узкую правку без delegation.

## Slice 1. Диагностика размера и packaging path
Цель: подтвердить фактический размер hook-бинаря и понять, на каком участке bundle он становится слишком большим.

Файлы/каталоги:
- `images/hooks/*`
- `.werf/stages/bundle.yaml`
- `werf.yaml`
- reference-проекты вне репозитория только для чтения

Проверки:
- локальная сборка hook-бинаря и измерение размера;
- чтение reference packaging flow в `virtualization` и `gpu-control-plane`.

Артефакт результата:
- подтверждённая причина oversized chart file.

## Slice 2. Узкая правка build/package flow
Цель: убрать избыточность из hook artifact или packaging path без архитектурного drift.

Файлы/каталоги:
- `images/hooks/*`
- `.werf/stages/bundle.yaml` и/или `werf.yaml`
- при необходимости `DEVELOPMENT.md`

Проверки:
- локальная проверка размера итогового hook artifact;
- релевантная `werf`-проверка или эквивалентный local build check.

Артефакт результата:
- bundle-compatible hooks packaging.

## Slice 3. Repo-level валидация
Цель: подтвердить, что правка не сломала module shell.

Файлы/каталоги:
- затронутые выше
- `plans/active/fix-hooks-chart-size-limit/*`

Проверки:
- `make helm-template`
- `make verify` если реализуемо

Артефакт результата:
- зелёный verify loop и готовый diff.

## Rollback point
Если выяснится, что причина не в hooks packaging, а в внешнем module-runtime contract или artifact format DKP, откатиться к диагностике без изменения layout и зафиксировать findings в bundle.

## Final validation
- `make helm-template`
- `make verify`
