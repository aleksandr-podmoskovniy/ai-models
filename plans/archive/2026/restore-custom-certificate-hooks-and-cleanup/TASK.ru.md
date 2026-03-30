# Вернуть CustomCertificate через hooks/batch и дочистить dead hooks tree

## Контекст
После попытки быстро снять ограничение по размеру chart artifact из модуля был удалён `images/hooks/*`, а support для `global.modules.https.mode=CustomCertificate` временно отключён через validate fail. Это убрало текущий startup blocker, но сломало нормальный DKP path для копирования custom certificate и оставило репозиторий в промежуточном состоянии с мёртвым layout drift.

Reference-проекты и Deckhouse shell helpers показывают два допустимых паттерна:
go/batch hooks и shell hooks. Для внешнего `ai-models` go/batch path оказался
слишком тяжёлым для Helm chart file limit, поэтому поддержку custom certificate
нужно вернуть через маленький shell hook wrapper поверх Deckhouse `shell_lib`.

## Постановка задачи
Перевести поддержку custom certificate на корректный DKP batch hooks pattern, удалить лишний `images/hooks` subtree окончательно и вернуть рабочий `CustomCertificate` flow без регресса в module packaging.

## Scope
- оформить и подключить top-level shell hook для common flow `copy_custom_certificate`;
- убрать остатки `images/hooks` и ссылок на устаревший image-based hooks flow;
- снять временный запрет на `global.modules.https.mode=CustomCertificate`;
- обновить bundle/layout/docs под новый канонический hooks path;
- прогнать узкие проверки, подтверждающие render/build contract.

## Non-goals
- не менять contract S3/PostgreSQL/auth поверх уже идущих задач, кроме необходимого касания docs;
- не вводить phase-2 controller/API логику;
- не перепаковывать backend image или upstream engine без необходимости;
- не пытаться сейчас полностью решить historical `kubeconform` hanging, если новый slice его не ухудшает.

## Затрагиваемые области
- `hooks/*`
- `.werf/stages/*`
- `templates/_helpers.tpl`
- `openapi/values.yaml` при необходимости для internal wiring
- `docs/development/REPO_LAYOUT.ru.md`
- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`
- `DEVELOPMENT.md`
- `plans/active/restore-custom-certificate-hooks-and-cleanup/*`

## Критерии приёмки
- в репозитории больше нет active runtime code под `images/hooks`;
- `CustomCertificate` снова не режется в validate и копируется через лёгкий shell hook pattern;
- bundle не содержит oversized hooks binary и больше не упирается в Helm chart file limit;
- `make lint`, `make test` и `make helm-template` проходят;
- структура репозитория становится чище и ближе к reference DKP modules.

## Риски
- перенос hooks в другой packaging path может потребовать корректировок `werf` stages и docs одновременно;
- common hook ожидает правильный internal values contract и path внутри bundle;
- неаккуратная cleanup-правка может конфликтовать с уже внесёнными изменениями по storage/secrets contract.
