# PLAN: добавить helper для загрузки Hugging Face модели

## Current phase

Этап 1. Managed backend inside the module. Это вспомогательный operator-facing
tooling slice, не phase-2 API design.

Режим orchestration: `solo`.

## Slices

### Slice 1. Добавить phase-1 helper script

Цель:
- дать минимальный runnable путь для Hugging Face model upload в текущий
  internal backend.

Файлы/каталоги:
- `tools/upload_hf_model.py`

Проверки:
- `python3 -m py_compile tools/upload_hf_model.py`

Артефакт результата:
- самодостаточный script с argparse и понятным stdout handoff.

### Slice 2. Проверить repo и дать UX clarification

Цель:
- подтвердить, что helper не ломает repo и не подменяет phase-2 contract.

Файлы/каталоги:
- `plans/active/add-hf-upload-helper/*`

Проверки:
- `make verify`

Артефакт результата:
- готовый helper и короткое объяснение:
  - сейчас загрузка идёт через helper/SDK;
  - дальше platform UX будет строиться через `Model` / `ClusterModel`.

## Rollback point

После Slice 1. Если script окажется слишком спорным, его можно удалить без
влияния на runtime contract модуля.

## Final validation

- `python3 -m py_compile tools/upload_hf_model.py`
- `make verify`
