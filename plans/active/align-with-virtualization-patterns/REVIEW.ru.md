# Review: `ai-models` vs `virtualization` alignment

## Что проверено

- reused canonical active bundle, без нового sibling source of truth;
- audit сравнил `ai-models` не только с `virtualization`, но и с
  `gpu-control-plane`, когда нужно было отделить stable DKP module pattern от
  repo-specific variation;
- подтверждённый module-shell drift был исправлен bounded change:
  `.werf/stages/bundle.yaml` теперь включает `monitoring/`, а
  `docs/development/REPO_LAYOUT.ru.md` фиксирует это как durable rule;
- continuation по controller/runtime действительно дошёл до implementation
  comparison, а не остался на уровне wording:
  проверены entrypoints, composition root, owner controllers, watch/indexer
  discipline, collector boundary и hotspot files;
- confirmed controller/runtime drifts тоже были исправлены bounded code changes:
  - manager/controller defaults в `bootstrap` теперь заданы явно, а не через
    implicit controller-runtime defaults;
  - `catalogstatus` переведён на metadata-only pod watch там, где mapping path
    использует только owner metadata;
- continuation по hook-owned secret sync тоже исправлен bounded change:
  `sync_artifacts_secrets` теперь namespace-aware и больше не пытается создать
  Secret в `d8-ai-models` до появления самого namespace;
- `images/controller/STRUCTURE.ru.md` теперь синхронизирован с live tree и
  явно разделяет:
  - production patterns, которые совпадают с `virtualization`;
  - intentional differences;
  - документный drift, который требовал исправления.

## Проверки

- `git diff --check`
- `werf config render --dev`
- `make verify`
- `cd images/hooks && go test ./pkg/hooks/sync_artifacts_secrets`

## Findings

Blocking findings нет.

Non-blocking findings:

- forced controller/runtime rewiring под `virtualization` не потребовался.
  Текущее `ai-models` tree уже удерживает production-grade boundaries и в ряде
  мест строже reference repo режет shell/config/bootstrap concerns.
- прямое копирование virtualization-style template-side secret rendering тоже
  не потребовалось: current fix оставляет credentials bytes в Secret-owned path
  и чинит именно namespace lifecycle safety.
- remaining differences from `virtualization` остаются defendable:
  они объясняются module-local ownership и отсутствием достаточного cross-owner
  reuse, а не недоделанным runtime shell.

## Residual risk

- reference parity по-прежнему decision-based, а не line-by-line cloning:
  если `virtualization` позже выделит новый reusable cross-owner seam, это не
  автоматически означает, что `ai-models` должен копировать его без local
  justification;
- текущий документ зафиксировал intentional differences, но при следующих
  runtime refactor'ах их всё равно нужно заново проверять, чтобы doc не начал
  снова отставать от кода.
