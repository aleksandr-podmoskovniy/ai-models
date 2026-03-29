# Выравнивание base_images pattern и ecosystem skills/subagents

## Контекст

Сейчас в `ai-models` уже есть рабочий `werf` shell и repo-local ecosystem для Codex, но в двух местах baseline получился слишком локальным:
- `base_images.yml` хранит только вручную отобранный подмножество образов вместо полного Deckhouse source-of-truth с автоматическим отсевом нужного во время сборки;
- skills/subagents описаны полезно, но пока остаются слишком ad-hoc для переиспользования в новых DKP модулях без повторения старых ошибок.

Пользователь просит привести это к более объективному и reusable виду по референсам из `virtualization`.

## Постановка задачи

Нужно:
- перевести `ai-models` на pattern полного Deckhouse image map с аккуратным build-time filtering нужных образов;
- систематизировать repo-local ecosystem skills/subagents так, чтобы она выглядела как reusable baseline для новых DKP модулей, а не как набор разрозненных локальных ролей.

## Scope

- изучить и перенести релевантный pattern из `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization`;
- выровнять `base_images` layout и `werf` wiring в `ai-models`;
- убрать явно случайные или избыточные элементы из текущих skills/agents;
- уточнить docs/context так, чтобы дальнейшее использование skills/subagents было более предсказуемым и объективным.

## Non-goals

- не менять runtime architecture MLflow;
- не переписывать deployment templates без прямой необходимости;
- не превращать `ai-models` в отдельный framework repo;
- не изобретать сложную orchestration system поверх встроенных Codex agent/skill механизмов.

## Затрагиваемые области

- `plans/align-base-images-and-agent-ecosystem/`
- `base_images.yml` или новый каталог для полного image map baseline
- `.werf/stages/`
- `werf.yaml`
- `.agents/skills/`
- `.codex/agents/`
- `.codex/README.md`
- при необходимости `AGENTS.md`, `DEVELOPMENT.md`

## Критерии приёмки

- `ai-models` использует полный Deckhouse base images source-of-truth и build-time filtering только реально используемых образов;
- layout и naming этого решения совпадают с DKP/virtualization pattern, а не с локальным самодельным вариантом;
- repo-local skills/subagents сведены к понятному и reusable baseline для новых DKP module repos;
- убраны очевидные patchwork-решения и лишний шум;
- `make verify` проходит;
- результат понятен как инженерная система, а не как набор персональных напоминаний.

## Риски

- перенос full image map может разрастить diff и потребовать аккуратного `werf` wiring;
- слишком агрессивная чистка skills/subagents может потерять полезные repo-specific guardrails;
- нельзя смешать reusable baseline с phase-2/phase-3 деталями текущего модуля.
