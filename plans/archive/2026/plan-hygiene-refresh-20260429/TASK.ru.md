# Plan hygiene refresh

## 1. Заголовок

Актуализировать `plans/active` после закрытия нескольких implementation
slice'ов.

## 2. Контекст

В active остались bundle'ы, которые уже содержат выполненные slice'ы и больше
не являются рабочей поверхностью. Такие bundle'ы засоряют выбор следующего
шага и заставляют повторно читать исторический контекст.

## 3. Задача

- классифицировать каждый active bundle;
- оставить в active только workstream'ы с конкретным next executable slice;
- перенести завершённые bundle'ы в `plans/archive/2026/`;
- обновить disposition в оставшихся active планах.

## 4. Non-goals

- Не менять runtime/API/templates/code.
- Не запускать live e2e.
- Не менять governance surfaces или skills/agents.

## 5. Критерии приёмки

- `plans/active` содержит только актуальные executable bundle'ы.
- Закрытые bundle'ы перенесены в архив.
- Оставшиеся active планы не ссылаются на архивированные bundle'ы как на
  текущие.
- `git diff --check` проходит.
