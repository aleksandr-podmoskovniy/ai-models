# A30 RayService cutover to ai-models registry

## 1. Заголовок

Перевод A30 embedder, reranker и STT RayService workloads на ai-models catalog
и внутренний registry-backed delivery.

## 2. Контекст

В `k8s.apiac.ru` уже выкачена новая версия `ai-models`. В соседнем GitOps repo
есть live RayService manifests для A30:

- `11-a30-embed-rayservice.yaml`
- `13-a30-stt-rayservice.yaml`
- `14-a30-rerank-rayservice.yaml`

Сейчас они используют собственную модель доставки моделей. Нужно загрузить
соответствующие модели через `ai-models`, перевести workloads на managed
delivery contract и проверить реальную нагрузку.

## 3. Постановка задачи

- Найти фактические model IDs / локальные пути в A30 manifests.
- Создать/актуализировать `ClusterModel` или `Model` resources для embedder,
  reranker и STT.
- Дождаться публикации моделей в ai-models registry.
- Перевести RayService pod templates на `ai.deckhouse.io/clustermodel` /
  `ai.deckhouse.io/model` и workload-facing env/path contract.
- Проверить rollout, логи, метрики, рестарты и реальные inference requests.

## 4. Scope

- Local GitOps chart:
  `/Users/myskat_90/Обучение/gitlab.ap.com/k8s-config/argo-projects/k8s.apiac.ru/kube-ray/charts/ray-service`
- Live cluster context from `/Users/myskat_90/.kube/k8s-config`
- ai-models `Model` / `ClusterModel` resources
- RayService workloads for A30 embedder, reranker and STT
- Test/evidence notes in this bundle

## 5. Non-goals

- Не менять ai-models controller code в этом slice.
- Не менять GPU/MPS placement policy, если A30 workloads уже стартуют.
- Не переводить GPT/LLM/example manifests вне A30 embedder/reranker/STT.
- Не делать production Git push без отдельной команды пользователя.

## 6. Затрагиваемые области

- `ray-service` chart manifests in external `k8s-config` repo.
- Live `kuberay-projects` namespace resources.
- ai-models catalog resources in cluster.

## 7. Критерии приёмки

- Три модели опубликованы через ai-models и имеют Ready artifact digest.
- A30 RayService pod templates ссылаются на ai-models model delivery contract,
  а не на legacy direct model path/download path.
- RayService rollout проходит без рестартов controller/DMCR и без постоянных
  worker restarts.
- Embedder/reranker/STT endpoints отвечают на реальные запросы.
- Проверены события workloads, pod logs и базовые метрики/рестарты.

## 8. Риски

- Некоторые модели могут быть gated/private на Hugging Face и потребуют
  существующий HF token secret.
- RayService containers могут ожидать фиксированный `model_id` или path, не
  совпадающий с `AI_MODELS_MODEL_PATH`.
- Большая публикация может упереться в artifact storage или node-cache capacity.
- GitOps controller может перезаписать manual live patches до изменения chart.
