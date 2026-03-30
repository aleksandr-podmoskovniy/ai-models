# Исправить local werf build для hooks image после gitArchive stage

## Контекст

После фикса `GOPROXY` локальный `werf build --dev --platform=linux/amd64`
проходит дальше и падает на `go-hooks-artifact`: install stage не может получить
commit info от собственного `gitArchive` stage.

Reference-модули `gpu-control-plane` и `virtualization` строят hooks иначе:
через отдельный `*-src-artifact` image и последующий import в builder image.

## Постановка задачи

Нужно выровнять сборку `images/hooks` под устойчивый DKP pattern, чтобы local
`werf` build не зависел от собственного `gitArchive` stage builder image.

## Scope

- `plans/active/fix-werf-hooks-gitarchive-stage/`
- `images/hooks/werf.inc.yaml`
- при необходимости `DEVELOPMENT.md`

## Non-goals

- не менять runtime поведение hooks;
- не перестраивать остальные images без необходимости;
- не чинить следующие blockers до тех пор, пока hooks image не проходит дальше.

## Критерии приёмки

- hooks image строится через отдельный source artifact/import pattern;
- local `werf build --dev --platform=linux/amd64` проходит дальше текущего
  `go-hooks-artifact/gitArchive` blocker;
- `make verify` проходит.

## Риски

- можно сломать bundle import path `/hooks/go`;
- можно разойтись с текущими image naming conventions.
