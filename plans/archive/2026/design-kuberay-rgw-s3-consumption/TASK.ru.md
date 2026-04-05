# Дизайн доступа KubeRay к model artifacts в RGW S3

## Контекст

`ai-models` уже умеет импортировать модели в MLflow backend и хранить реальные model artifacts в RGW/S3. Следующий практический шаг — дать serving plane (`KubeRay` / `Ray Serve LLM`) читать эти artifacts напрямую из S3.

Текущий `RayService` values в `k8s-config/.../kube-ray/charts/ray-service/ap-values.yaml` всё ещё настроен на прямую загрузку модели из Hugging Face и не прокидывает `AWS_*` credentials, `AWS_CA_BUNDLE` и `s3://...` model source.

## Постановка задачи

Понять и зафиксировать каноничный для текущего этапа способ:
- какой вид аутентификации использовать в RGW для serving plane;
- как прокинуть credentials и CA в `RayService`;
- какой `artifact URI` выдавать из MLflow в `KubeRay`;
- какие ограничения есть у более продвинутого варианта с temporary credentials / STS.

## Scope

- Проверить текущий `RayService` values и service account wiring.
- Проверить официальный flow `Ray Serve LLM` для загрузки модели из object storage/S3.
- Проверить официальный flow `Ceph RGW` для long-lived S3 creds и STS/WebIdentity.
- Подготовить конкретную рекомендуемую конфигурацию для `RayService`.
- Если конфигурация в `ap-values.yaml` может быть безопасно улучшена механически, подготовить patch.

## Non-goals

- Не проектировать здесь новый публичный DKP API каталога моделей.
- Не менять backend deployment shape `ai-models`.
- Не пытаться в этом slice внедрить полноценный STS federation без подтверждённого RGW/Dex PoC.
- Не реализовывать здесь training orchestration.

## Затрагиваемые области

- `plans/active/design-kuberay-rgw-s3-consumption/*`
- внешний deployment config:
  - `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s.apiac.ru/kube-ray/charts/ray-service/ap-values.yaml`
  - `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s.apiac.ru/kube-ray/charts/ray-service/01-ServiceAccount.yaml`

## Критерии приёмки

- Есть однозначный ответ, какие credentials нужны `KubeRay` для доступа к RGW.
- Есть однозначный ответ, какой именно URI из MLflow надо отдавать в serving plane.
- Есть конкретный пример `RayService` wiring для `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, optional `AWS_SESSION_TOKEN` и `AWS_CA_BUNDLE`.
- Ясно зафиксировано, что `MLflow SSO` не используется как serving auth.
- Если предложен STS/WebIdentity путь, он описан как future/hardening path с явной оговоркой о необходимости PoC.

## Риски

- `Ray Serve LLM` может ожидать не прямой `s3://` model source, а другой storage contract в конкретной версии `Ray`.
- `RGW STS` может работать не полностью совместимо с текущим `Dex` без отдельной настройки OIDC trust.
- Внешний chart может требовать одинакового env/mount wiring и для head, и для workers; неполный patch даст неочевидные runtime failures.
