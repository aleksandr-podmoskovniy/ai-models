# PLAN

## 1. Current phase

Этап 3 style hardening поверх phase-1 backend packaging: задача не меняет backend contract, а чинит build/release discipline вокруг upstream patching.

## 2. Orchestration

`solo`

Причина: узкий и прямой build-layout bugfix без архитектурной неопределённости.

## 3. Slices

### Slice 1. Align werf final backend image layout

- Цель: добавить пропущенный import `backend-oidc-auth-ui-build -> /oidc-auth-ui` в финальный `backend` image.
- Файлы:
  - `images/backend/werf.inc.yaml`
- Проверки:
  - локальный targeted diff review
- Результат:
  - `install-oidc-auth-from-source.sh` в werf final stage видит prebuilt UI assets.

### Slice 2. Add guard for werf layout contract

- Цель: поймать такой разрыв до CI.
- Файлы:
  - `Makefile`
- Проверки:
  - `make verify`
- Результат:
  - verify включает check на согласованность werf OIDC UI import layout.

## 4. Rollback point

До правки `images/backend/werf.inc.yaml`: можно откатиться к предыдущему состоянию без влияния на runtime, останется только известный CI blocker.

## 5. Final validation

- `make verify`
