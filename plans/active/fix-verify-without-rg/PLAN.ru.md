## 1. Current phase
Этап 1. Managed backend inside the module.

## 2. Slices

### Slice 1. Убрать зависимость verify от rg
- Цель: заменить `rg` в `backend-runtime-entrypoints-check` на доступную shell-проверку.
- Файлы/каталоги: `Makefile`
- Проверки:
  - `make backend-runtime-entrypoints-check`
  - `make verify`
- Результат: verify-путь работает в CI runner без `ripgrep`.

## 3. Rollback point
До изменения `Makefile` текущее поведение легко восстанавливается одной обратной правкой.

## 4. Final validation
- `make backend-runtime-entrypoints-check`
- `make verify`
