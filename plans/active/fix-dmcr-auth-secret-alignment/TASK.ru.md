## 1. Заголовок

Выравнивание `DMCR` auth secrets и устранение render-time password drift

## 2. Контекст

Live `HuggingFace` publish для `Gemma 4` доходит до source mirror и native OCI
publish, но падает на `DMCR` basic auth. Проверка живого кластера показала, что
`Secret/d8-ai-models/ai-models-dmcr-auth` и projected
`ai-models-dmcr-auth-write` / `ai-models-dmcr-auth-read` содержат разные
пароли, хотя должны описывать один и тот же server/client auth contract.

Причина находится в template layer: `lookup` не видит секрет, который
рендерится в этом же проходе, а helper c `randAlphaNum` вызывается несколько
раз в одном `helm template`, из-за чего server auth secret и dockerconfigjson
secrets могут получить разные generated passwords.

## 3. Постановка задачи

Нужно устранить render-time drift в генерации `DMCR` auth secrets так, чтобы
server-side htpasswd secret и projected read/write dockerconfigjson secrets
всегда использовали один и тот же пароль в рамках одного render/install, а
дополнительно зафиксировать это machine-checkable render validation.

После repo fix нужно не только устранить first-install drift, но и сделать
upgrade-safe recovery для уже битых кластеров: при первом render после rollout
server-side `htpasswd` entries должны пересобираться из текущих
`write.password` / `read.password`, если старый secret ещё не несёт нового
alignment marker. Только после этого имеет смысл повторять publication smoke на
`HuggingFace` модели `Gemma 4`.

## 4. Scope

- исправить template generation `DMCR` auth secrets без redesign auth model;
- убрать множественные независимые password generation paths в одном render;
- сделать reuse existing server-side `htpasswd` entries upgrade-safe, чтобы
  старые кластеры с исторически битым bcrypt drift self-heal'ились после
  module rollout без ручной пересборки секрета;
- добавить render-level guardrail, который ловит drift между:
  - `ai-models-dmcr-auth`
  - `ai-models-dmcr-auth-write`
  - `ai-models-dmcr-auth-read`
- повторно проверить live `HF -> source mirror -> DMCR publish` smoke после
  выравнивания cluster secrets.

## 5. Non-goals

- не менять usernames, registry realm или `DMCR` auth backend;
- не перепроектировать `DMCR` deployment/config shell;
- не менять public `Model` / `ClusterModel` contract;
- не вводить новые values knobs для auth;
- не делать в этом slice новый secret rotation protocol поверх старых
  паролей; recovery должен reuse'ить уже выбранные `write.password` /
  `read.password`.

## 6. Затрагиваемые области

- `templates/_helpers.tpl`
- `templates/dmcr/secret.yaml`
- `tools/helm-tests/validate-renders.py`
- `tools/helm-tests/validate_renders_test.py`
- `plans/active/fix-dmcr-auth-secret-alignment/*`

## 7. Критерии приёмки

- в одном `helm template` server auth secret и projected write/read secrets
  используют одинаковые пароли для соответствующего пользователя;
- `write.htpasswd` и `read.htpasswd` остаются согласованными со своими
  `*.password` полями;
- upgraded render для уже существующего server auth secret без нового
  alignment marker пересобирает `write.htpasswd` / `read.htpasswd` из текущих
  `write.password` / `read.password`, а не blindly reuse'ит старый bcrypt;
- render validation падает, если:
  - `ai-models-dmcr-auth.write.password` не совпадает с
    `ai-models-dmcr-auth-write.password`;
  - `ai-models-dmcr-auth.read.password` не совпадает с
    `ai-models-dmcr-auth-read.password`;
  - server auth secret не несёт нового machine-checkable alignment marker для
    `write` / `read` htpasswd entries;
  - `dockerconfigjson` не совпадает с ожидаемыми username/password значениями;
- render validation и его focused test проходят на clean runner без
  внешней Python-зависимости `PyYAML`;
- `make helm-template` продолжает проходить;
- после rollout обновлённого module build live `Gemma 4` publish smoke проходит
  дальше `DMCR` auth boundary и не падает на `401 authentication failure`.

## 8. Риски

- можно сломать reuse existing secrets на upgrade, если helper перестанет
  корректно читать уже существующие `DMCR` auth secrets;
- можно добавить слишком узкую render validation, которая будет ломаться на
  допустимом future refactor без реального runtime drift;
- live cluster retry может потребовать ручного выравнивания secrets до rollout
  обновлённого модуля.
