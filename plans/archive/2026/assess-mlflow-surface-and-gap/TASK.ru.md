# Оценить реальный MLflow surface и gap phase-1 модуля

## Контекст

`ai-models` уже поднимает внутренний managed backend на базе upstream `MLflow`.
После запуска в кластере стало видно, что UI и API содержат заметно больше
возможностей, чем было явно оформлено как phase-1 contract модуля: gateway API,
prompts, scorers, traces, datasets, logged models и другие поверхности.

Пользователь хочет понять:
- точно ли сейчас работает SSO;
- как practically использовать backend для registry use cases;
- как грузить model artifacts, включая Hugging Face сценарии;
- что из видимого upstream surface мы сознательно поддерживаем, а что пока нет;
- не недоиспользуем ли мы `MLflow` и не прячем ли полезные возможности зря.

## Постановка задачи

Собрать проверяемую картину по трём слоям:
- что реально работает в живом кластере сейчас;
- что модуль действительно wiring'ит и поддерживает как phase-1 contract;
- какие дополнительные возможности есть в upstream `MLflow`, но пока не
  platformized и не должны считаться supported contract модуля.

## Scope

- Проверить текущий SSO path в кластере.
- Проверить реальные runtime endpoints и логи backend.
- Сверить текущий repo wiring auth/storage/runtime c phase-1 целями модуля.
- Сверить доступные upstream возможности `MLflow` с тем, что реально exposed.
- Зафиксировать supported / unsupported / future-possible capability matrix.
- Дать практический вывод по registry flow, включая Hugging Face artifacts.

## Non-goals

- Не проектировать app-native OIDC parity с `n8n`.
- Не менять сейчас runtime contract модуля без отдельной реализации.
- Не тащить в phase 1 публичный DKP API `Model` / `ClusterModel`.
- Не объявлять всё видимое в UI поддержанным platform contract без отдельного
  дизайна и hardening.

## Затрагиваемые области

- `plans/active/assess-mlflow-surface-and-gap/*`
- cluster runtime (`d8-ai-models`, `user-authn`, ingress)
- auth и runtime templates:
  - `templates/auth/`
  - `templates/backend/`
- docs phase framing:
  - `docs/development/PHASES.ru.md`
  - `docs/CONFIGURATION*.md`

## Критерии приёмки

- Есть подтверждение, работает ли текущий SSO path и в каком именно режиме.
- Есть разделение между ingress-level SSO и app-native OIDC capabilities.
- Есть понятный список phase-1 supported capabilities и явных gap'ов.
- Есть практический ответ, как использовать backend для registry flow и Hugging
  Face model artifacts без придумывания несуществующего UI workflow.
- Выводы оформлены так, чтобы по ним можно было принять решение, что включать в
  roadmap дальше, а что сознательно не поддерживать.

## Риски

- Легко перепутать "видно в upstream UI" и "поддерживается модулем".
- Можно преждевременно размыть phase-1 scope и начать обещать phase-2/3 вещи.
- Cluster runtime может показывать больше surface, чем сейчас безопасно
  platformize без hardening и ownership model.
