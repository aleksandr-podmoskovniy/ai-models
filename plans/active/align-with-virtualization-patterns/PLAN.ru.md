# PLAN: Сверка `ai-models` с production patterns модуля `virtualization`

## Current phase

Задача относится к phase-2/phase-3 engineering discipline:

- public/runtime/catalog surfaces уже существуют;
- нужно проверить, что module shell и implementation boundaries не ушли от
  production-grade DKP patterns, на которые repo сам ссылается;
- работа не должна тянуть новый product/API design, только alignment и
  hardening текущих boundaries.

## Orchestration

- режим: `solo`
- причина:
  - сначала нужен цельный audit matrix across multiple module surfaces;
  - реализация может остаться review-only или потребовать только bounded
    alignment after findings;
  - текущий execution policy не предполагает неявную delegation, поэтому
    read-only architect review выполняется main agent locally с фиксацией
    выводов в bundle.

## Slices

### Slice 1. Build audit matrix against `virtualization`

Цель:

- снять reference patterns из `../virtualization`;
- сравнить их с `ai-models` по module shell, `werf`, build, `templates`,
  `openapi`, `images`, workflow и runtime ownership seams;
- зафиксировать drift matrix и candidate actions.

Файлы/каталоги:

- `plans/active/align-with-virtualization-patterns/*`
- reference reads from `../virtualization/*`
- current reads from repo root, `docs/development/*`, `images/controller/*`

Проверки:

- focused tree/file inspection
- explicit diff notes in bundle

Артефакт:

- audit notes or direct findings recorded in the bundle with action classes
  `align now` / `intentional difference` / `defer`.

Continuation requirement for this slice:

- separate module-shell findings from controller/runtime implementation
  findings;
- explicitly recheck `images/controller/STRUCTURE.ru.md` against the live tree
  and the production patterns extracted from `../virtualization`.

### Slice 2. Realign production drifts

Цель:

- исправить только те расхождения, которые подтверждены audit'ом как real
  production drift relative to `virtualization` patterns.

Файлы/каталоги:

- bounded subset of:
  - root `werf.yaml`, `.werf/*`, `build/*`, `.github/workflows/*`
  - `docs/development/REPO_LAYOUT.ru.md`, `docs/CONFIGURATION*.md`
  - `templates/*`, `openapi/*`, `images/*`

Проверки:

- targeted checks per touched surface
- `git diff --check`

Артефакт:

- implementation/docs aligned to approved production patterns.

Continuation requirement for this slice:

- if controller/runtime code is already aligned, still update
  `images/controller/STRUCTURE.ru.md` so docs stop lagging behind code;
- if only doc drift is confirmed, keep code unchanged and record the
  implementation differences as intentional instead of forcing superficial
  parity with `virtualization`.

### Slice 3. Validate and review

Цель:

- подтвердить, что alignment не привнёс patchwork и не оставил docs drift.

Файлы/каталоги:

- touched files from previous slices
- `plans/active/align-with-virtualization-patterns/*`

Проверки:

- `make verify`
- `git diff --check`

Артефакт:

- final review notes in bundle and verified repo state.

## Rollback point

После Slice 1:

- есть только audit bundle и findings;
- code/build/runtime surfaces ещё не менялись;
- можно остановиться без repository drift beyond planning artifacts.

## Final validation

- `git diff --check`
- `make verify`
