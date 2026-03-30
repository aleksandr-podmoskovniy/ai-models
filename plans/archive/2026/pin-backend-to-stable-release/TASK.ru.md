# Перепинить backend upstream на последний стабильный release

## Контекст

Сейчас internal backend source pinned на dev snapshot из upstream `MLflow`:
`images/backend/upstream.lock` указывает на commit из `main` и версию
`3.11.1.dev0`.

Для production-oriented DKP module это неверный baseline: build должен идти от
последнего стабильного upstream release tag, а не от development branch.

## Постановка задачи

Нужно перевести upstream pinning и repo-facing metadata на последний стабильный
release `MLflow`, сохранив reproducible fetch/build flow.

## Scope

- `plans/active/pin-backend-to-stable-release/`
- `images/backend/upstream.lock`
- при необходимости `DEVELOPMENT.md`
- при необходимости `images/backend/patches/README.md`

## Non-goals

- не менять runtime templates модуля;
- не добавлять новые patch files;
- не менять backend build pipeline сверх того, что нужно для stable release pin.

## Затрагиваемые области

- metadata pinned upstream source;
- docs по fetch/rebase process;
- локальная validate loop для source fetch.

## Критерии приёмки

- `images/backend/upstream.lock` указывает на последний стабильный upstream
  release, а не на dev snapshot;
- version/image tags больше не используют `dev0`;
- local source fetch подтверждает locked stable version;
- repo-facing docs не вводят в заблуждение относительно release policy.

## Риски

- stable tag может не совпасть с текущими assumptions build shell;
- upstream stable tag может потребовать другой submodule layout или paths;
- docs могут остаться в mixed state, если обновить только metadata.
