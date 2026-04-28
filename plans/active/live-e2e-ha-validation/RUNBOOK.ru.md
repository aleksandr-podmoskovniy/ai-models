# Runbook: комплексный live e2e/HA

Этот файл — рабочая шпаргалка для старта по команде. Он не заменяет
`TASK.ru.md` и `PLAN.ru.md`; здесь только порядок выполнения и evidence gates.

## Перед стартом

- Подтвердить context: `k8s.apiac.ru`.
- Подтвердить, что local `main` соответствует build, выкаченному в кластер.
- Проверить, что нет активных пользовательских `Model`/`ClusterModel`, которые
  можно спутать с e2e.
- Создать/проверить namespace `ai-models-e2e`.
- Все test resources маркировать:
  `ai.deckhouse.io/live-e2e=ha-validation`.

## Минимальная модельная матрица

Подбирать публичные маленькие модели в момент запуска, потому что HF state
может меняться.

Обязательные классы:

- `Safetensors` text/chat.
- `GGUF` text/chat.
- `Diffusers` text-to-image.
- `Diffusers` text-to-video или image-to-video, если есть достаточно маленький
  публичный artifact.
- Embeddings.
- Rerank.
- STT.
- TTS.
- CV image classification/object detection/segmentation.
- Multimodal vision-language.
- Tool-calling capable chat template.

Если класс невозможно опубликовать из-за unsupported artifact layout, pass
только при явном отказе до тяжёлого byte path и корректном status reason.

## Evidence gates

Для каждого scenario фиксировать:

- resource YAML before/after;
- events;
- controller logs;
- source-worker/upload-gateway logs;
- DMCR logs;
- direct-upload state secret phase/stage без секретных значений;
- storage usage/capacity metrics or explicit absence;
- cleanup/GC result.

## RBAC matrix gate

RBAC проверяется до destructive HA шагов и повторяется после rollout, если
шаблоны менялись. Сравнение с virtualization:

- module-specific `user-authz` правила должны быть только delta-фрагментами с
  `user-authz.deckhouse.io/access-level`;
- inheritance между `User` -> `PrivilegedUser` -> `Editor` -> `Admin` и
  cluster-level ролями обеспечивает Deckhouse `user-authz`, а не копирование
  одних и тех же правил в каждом module fragment;
- пустой fragment допустим только если для этого уровня нет нового
  module-owned действия.

Проверить first-version `user-authz`:

- `User`: allow `get/list/watch` `models` and `clustermodels`; deny
  `create/update/patch/delete`, `status`, `finalizers`, `secrets`, `pods/log`,
  `pods/exec`, `pods/attach`, `pods/portforward`, internal runtime objects.
- `PrivilegedUser`: same module delta as `User`; no extra module-local Secret
  or pod subresource access.
- `Editor`: allow write for namespaced `models`; deny write for
  `clustermodels`, deny sensitive paths.
- `Admin`: same module delta as `Editor`; no service-object delete surface is
  exposed by `ai-models`.
- `ClusterEditor`: allow write for `clustermodels`; also verify effective
  persona keeps namespaced model write through Deckhouse inheritance.
- `ClusterAdmin`: same module delta as `ClusterEditor`; no extra
  module-owned service-object surface is exposed.
- `SuperAdmin`: no module-specific fragment; verify effective global persona
  can operate while namespaceSelector/limitNamespaces semantics are not
  bypassed by module resources.

Проверить `rbacv2`:

- `d8:use:capability:module:ai-models:view`: allow read `models`; deny
  `clustermodels` and all sensitive paths.
- `d8:use:capability:module:ai-models:edit`: allow write `models`; deny
  `clustermodels`, `moduleconfigs`, sensitive paths.
- `d8:manage:permission:module:ai-models:view`: allow read
  `models`, `clustermodels` and `ModuleConfig/ai-models`; deny sensitive paths.
- `d8:manage:permission:module:ai-models:edit`: allow write
  `models`, `clustermodels` and `ModuleConfig/ai-models`; deny status,
  finalizers, Secrets and internal runtime objects.

Evidence commands:

- render/static: `make helm-template`;
- live matrix: `kubectl auth can-i --as=<e2e-subject> ...`;
- service-account boundary:
  `kubectl auth can-i --as=system:serviceaccount:d8-ai-models:ai-models-controller ...`
  and `--as=system:serviceaccount:d8-ai-models:dmcr ...`;
- record every allow/deny row in `NOTES.ru.md`.

## Stop conditions

Остановить прогон и перейти к fix-slice, если:

- public status получает terminal `Failed` на replayable interruption;
- upload reservation допускает превышение capacity;
- cleanup удаляет runtime state без request-scoped backend cleanup evidence;
- controller/upload-gateway после rollout остаются на master/control-plane;
- SharedDirect workload попадает на ноду без ready node-cache runtime;
- any human-facing role allows Secret, pod log/exec/attach/port-forward,
  `*/status`, `*/finalizers` or internal runtime objects unexpectedly;
- namespaced `use` grants `ClusterModel`, or cluster personas cannot operate
  `ClusterModel` through the intended path;
- logs не позволяют связать model -> worker -> direct-upload -> DMCR artifact.

## Cleanup

После каждого scenario удалять test resources, если следующий scenario не
использует их как dependency. После всего run:

- удалить namespace `ai-models-e2e`;
- удалить cluster-scoped test `ClusterModel`;
- вернуть временные `ModuleConfig` изменения;
- дождаться DMCR GC done или зафиксировать deliberate delay;
- проверить отсутствие e2e-labeled pods/secrets/jobs/leases.
