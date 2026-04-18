## 1. Current phase

Этап 2. `DMCR` уже является live internal publication backend, а задача
затрагивает runtime auth boundary внутри controller-owned publication path.

## 2. Orchestration

`solo`

Boundary уже узкая и технически ясная: проблема ограничена `DMCR` template
generation и render/runtime validation. Отдельные read-only subagents не
нужны: public API, module layout и внешние integration boundaries не
перепроектируются.

## 3. Slices

### Slice 1. Зафиксировать auth drift и target invariant

- Цель:
  - оформить task bundle и зафиксировать live failure mode как render-time
    password drift между server auth secret и projected registry auth secrets.
- Файлы:
  - `plans/active/fix-dmcr-auth-secret-alignment/*`
- Проверки:
  - `rg -n "dmcrWriteAuthPassword|dmcrWriteHTPasswdEntry|dmcrReadHTPasswdEntry|dmcrDockerConfigJSON" templates/_helpers.tpl templates/dmcr/secret.yaml`
- Артефакт:
  - bundle с явным runtime invariant: один render -> один пароль на
    пользователя во всех `DMCR` auth secrets.

### Slice 2. Убрать multi-generate drift из `DMCR` secret render

- Цель:
  - сделать password selection один раз на render и использовать его дальше
    для server/password, htpasswd и dockerconfigjson surfaces.
- Файлы:
  - `templates/_helpers.tpl`
  - `templates/dmcr/secret.yaml`
- Проверки:
  - `./tools/helm-tests/helm-template.sh`
- Артефакт:
  - `DMCR` auth secrets, которые больше не расходятся на первом install/render.

### Slice 3. Добавить machine-checkable render guardrail

- Цель:
  - чтобы drift ловился до runtime, а не на live `401`.
- Файлы:
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `python3 ./tools/helm-tests/validate-renders.py ./tools/kubeconform/renders`
  - `make helm-template`
- Артефакт:
  - render validator, который проверяет password/json consistency между
    `DMCR` auth secrets.

### Slice 4. Сделать existing `htpasswd` reuse upgrade-safe

- Цель:
  - чтобы rollout нового chart self-heal'ил уже битые first-install secrets, а
    не только новые render/install.
- Файлы:
  - `templates/_helpers.tpl`
  - `templates/dmcr/secret.yaml`
  - `tools/helm-tests/validate-renders.py`
- Проверки:
  - `make helm-template`
  - `python3 ./tools/helm-tests/validate-renders.py ./tools/kubeconform/renders`
- Артефакт:
  - server auth secret с явным alignment marker, по которому Helm helper
    понимает, можно ли reuse'ить старый bcrypt entry.

### Slice 5. Повторить live smoke для `HuggingFace Gemma 4`

- Цель:
  - после rollout модуля убедиться, что upgraded cluster self-heal'ит server
    auth secret и publish проходит `DMCR` auth boundary.
- Файлы:
  - без обязательных repo edits; при необходимости обновление bundle notes
- Проверки:
  - `kubectl -n d8-ai-models get secret ai-models-dmcr-auth ai-models-dmcr-auth-write ai-models-dmcr-auth-read -o yaml`
  - `kubectl -n d8-ai-models logs deploy/dmcr --since=15m`
  - `kubectl -n ai-models-smoke get model -o wide`
- Артефакт:
  - подтверждённый live smoke без `DMCR 401`.

## 4. Rollback point

После Slice 1 можно безопасно остановиться: failure mode уже зафиксирован, но
runtime/template behavior ещё не изменён.

После Slice 2 rollback тоже остаётся простым: render contract уже выровнен, но
guardrail и live retry ещё не обязательны для частичного отката.

## 5. Final validation

- `make helm-template`
- `make kubeconform`
- `make verify`
- live `HF Gemma 4` publish smoke без `DMCR` auth failure
