# Убрать неявную PyYAML-зависимость из render validator

## Контекст

`make verify` локально проходил, но в GitHub Actions падает на
`tools/helm-tests/validate-renders.py` с `ModuleNotFoundError: No module named 'yaml'`.

Причина — validator использовал `PyYAML`, хотя repo и CI не объявляют эту
зависимость как обязательную часть toolchain.

## Постановка задачи

Нужно сделать render validator самодостаточным: он должен работать на чистом
`python3` без сторонних пакетов и сохранять текущую semantic-проверку для
managed `Postgres`.

## Scope

- переписать `tools/helm-tests/validate-renders.py` без `PyYAML`;
- прогнать локальный `make verify`;
- зафиксировать короткий review.

## Non-goals

- не менять сам contract `managed-postgres`;
- не тянуть новый python dependency в CI;
- не расширять validator на другие custom resources в этом срезе.

## Критерии приёмки

- `validate-renders.py` не импортирует `yaml`;
- `make verify` проходит локально;
- GitHub Actions больше не требуют `PyYAML` для `make verify`.
