# Починить oversized go hooks chart artifact

## Контекст
При установке модуля `ai-models` startup падает на `helm install` с ошибкой `chart file "ai-models-hooks" is larger than the maximum file size 5242880`. Ошибка означает, что go-hooks bundle, попадающий в chart archive, превышает лимит Helm/Deckhouse packaging и делает модуль неустанавливаемым.

## Постановка задачи
Найти источник избыточного размера hook-артефакта и уменьшить его до рабочего размера без нарушения DKP layout и runtime contract модуля.

## Scope
- измерить фактический размер build-артефакта hooks;
- сравнить текущий hook packaging с reference-проектами `virtualization` и `gpu-control-plane`;
- поправить build/package flow hooks так, чтобы chart перестал превышать лимит размера;
- зафиксировать проверки, подтверждающие размер и работоспособность bundle.

## Non-goals
- не менять функциональную логику hooks без необходимости;
- не трогать backend/runtime templates, если проблема решается на уровне hooks packaging;
- не внедрять новые runtime зависимости только ради сжатия артефакта.

## Затрагиваемые области
- `images/hooks/*`
- `.werf/stages/*`, `werf.yaml`, `werf-giterminism.yaml` при необходимости
- `Makefile` или docs только если меняется verify/build workflow
- `plans/active/fix-hooks-chart-size-limit/*`

## Критерии приёмки
- локально подтверждён размер hook-бинаря/артефакта и источник превышения лимита;
- `ai-models-hooks` больше не превышает лимит 5 MiB в packaging path;
- `make helm-template` и релевантная `werf`-проверка проходят без нового regressions blocker;
- изменение укладывается в phase 1 module shell и не размывает layout.

## Риски
- уменьшение размера может потребовать изменения способа сборки hooks и повлиять на runtime startup;
- reference-проекты могут использовать неочевидный packing helper, который нельзя копировать механически.
