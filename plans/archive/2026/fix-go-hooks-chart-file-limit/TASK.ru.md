# Исправить install-path лимит на файл Go hooks в external module chart

## Контекст

После возврата `ai-models` к family-pattern `images/hooks -> go-hooks-artifact -> /hooks/go` repo-level проверки проходят, но на живом Deckhouse install падает с ошибкой `chart file "ai-models-module-hooks" is larger than the maximum file size 5242880`.

## Постановка задачи

Нужно найти реальный packaging/integration path, на котором Go hooks binary превращается в oversized chart file, и привести `ai-models` к рабочему решению по образцу соседних модулей без выдумывания новой архитектуры.

## Scope

- найти код или contract, где external module install накладывает лимит на chart files;
- сравнить install semantics `ai-models` с `gpu-control-plane` и `virtualization`;
- исправить packaging/wiring так, чтобы Go hooks доставлялись без oversized chart file error;
- прогнать релевантные проверки.

## Non-goals

- не менять phase-1 runtime contract backend/PostgreSQL/S3;
- не добавлять новые hooks кроме уже существующего `copy_custom_certificate`;
- не делать unrelated cleanup.

## Затрагиваемые области

- `images/hooks`
- `werf.yaml`
- `.werf/stages/*`
- docs/notes по hooks packaging
- `plans/active/fix-go-hooks-chart-file-limit/*`

## Критерии приёмки

- найден и описан конкретный источник лимита;
- hooks delivery выровнен с рабочим DKP path для external modules;
- repo checks проходят;
- нет root-level `batchhooks` workaround.

## Риски

- можно снова скатиться в workaround вместо семейного решения;
- cluster-side path может отличаться от локального `werf config render`, и это надо учитывать явно.
