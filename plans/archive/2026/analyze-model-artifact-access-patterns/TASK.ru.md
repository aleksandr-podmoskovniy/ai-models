# Analyze Model Artifact Access Patterns

## 1. Контекст

Для `Model` / `ClusterModel` уже вырисовывается flow:

- пользователь задаёт `spec.source`;
- модуль сохраняет модель во внутренний backend path или в OCI-based
  distribution path;
- контроллер публикует stable artifact reference и metadata в `status`;
- runtime consumer materializes модель в локальный PVC или shared volume.

Следующий архитектурный вопрос: как правильно выдавать доступ к опубликованным
артефактам для materialization agent.

Пользователь ожидает:

- для `payload-registry` и OCI path доступ должен идти через привычный
  Kubernetes/RBAC-паттерн, как для других container workloads;
- для S3-compatible path нужен корректный паттерн выдачи credentials во время
  работы агента, а не ручная раздача статических секретов;
- нужно понять, что принято считать правильным и что стоит фиксировать в нашем
  design.

## 2. Постановка задачи

Собрать и сформулировать рекомендуемую модель доступа к model artifacts для двух
delivery classes:

- OCI / `payload-registry`;
- S3-compatible object storage.

Нужно отделить:

- что является public/runtime contract;
- что остаётся internal implementation detail контроллера и backend;
- какой auth pattern стоит считать базовым и безопасным для materialization
  agent.

## 3. Scope

- локальные design/docs материалы `ai-models`;
- локальные референсы из `virtualization`;
- внешний анализ best practices по официальным источникам;
- bundle under `plans/active/analyze-model-artifact-access-patterns/*`.

## 4. Non-goals

- Не менять код или CRD в этом slice.
- Не фиксировать окончательный runtime implementation (`init-container`,
  `sidecar`, отдельный agent).
- Не проектировать сейчас полную security hardening story поверх digest/signature
  verification.

## 5. Затрагиваемые области

- `plans/active/analyze-model-artifact-access-patterns/*`
- локальные docs/reviews вокруг registry/backend access
- при необходимости итоговые notes/review в bundle

## 6. Критерии приёмки

- Есть чёткий вывод по recommended access pattern для OCI/payload-registry.
- Есть чёткий вывод по recommended access pattern для S3-compatible storage.
- Разделены public contract и internal credential issuance mechanics.
- Вывод соотнесён с паттернами virtualization и не конфликтует с ADR direction.

## 7. Риски

- Смешать platform contract с transport/auth implementation details.
- Предложить для S3 слишком слабый operational path с долгоживущими секретами.
- Ошибочно приравнять OCI artifact pull к generic S3 credential distribution.
