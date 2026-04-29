# Manual A30 vLLM SharedDirect drill

Цель — после выката новой версии модуля проверить самый важный path без
RayService и без GitOps: обычный `Deployment` с одной ai-models annotation
должен получить node-cache CSI volume, resolved artifact attributes, mounts и
env от ai-models.

Запрещено добавлять в workload:

- `materialize-artifact` init container;
- PVC `model-cache-pvc`;
- DMCR credentials/CA env;
- ручной `/data/modelcache` `mkdir`;
- digest/artifact URI руками.

Если нужны digest/artifact URI, Secret или materialize runtime в workload
manifest, тест считается проваленным: фиксить надо mutation / node-cache
contract. CSI volume с driver `node-cache.ai-models.deckhouse.io` должен
появиться только после mutation контроллера.

## 0. No-action gate

Не выполнять live mutation до подтверждения пользователя, что новая версия
модуля выкачена в `k8s.apiac.ru`.

```bash
export KUBECONFIG=/Users/myskat_90/.kube/k8s-config
export CTX=k8s.apiac.ru
export NS=ai-models-e2e
export TARGET_NODE=k8s-w3-gpu.apiac.ru
```

Проверка контекста:

```bash
kubectl --context "$CTX" config current-context
kubectl --context "$CTX" get moduleconfig ai-models -o yaml
kubectl --context "$CTX" -n d8-ai-models get deploy,pod -o wide
```

## 1. Preflight после выката

Проверить, что target node и локальные диски на месте:

```bash
kubectl --context "$CTX" get node "$TARGET_NODE" --show-labels
kubectl --context "$CTX" describe node "$TARGET_NODE" | grep -E 'Taints:|gpu.deckhouse.io|nvidia.com|a30|mig' || true
kubectl --context "$CTX" get blockdevices.storage.deckhouse.io -o wide
```

Ожидаемые факты на текущем кластере:

- `k8s-w3-gpu.apiac.ru`;
- taint `dedicated.apiac.ru=w-gpu:NoExecute`;
- label `node.deckhouse.io/group=w-gpu-mig`;
- BlockDevices:
  `dev-0b257c2c37d39bec8279efd49d0e7d315fddfdfe`,
  `dev-2c4adac3cce4860ce6686d249ef6a96bf268a2ea`;
- оба `consumable=true`, `150Gi`.

Если `gpu.deckhouse.io/a30-mig-1g6` не виден в allocatable после выката, это
не ai-models defect; сначала чинить GPU/DRA/device-plugin слой.

## 2. Включение node-cache substrate

Выполнять только после выката новой версии.

```bash
kubectl --context "$CTX" label node "$TARGET_NODE" \
  ai.deckhouse.io/model-cache=true --overwrite

kubectl --context "$CTX" label blockdevice.storage.deckhouse.io \
  dev-0b257c2c37d39bec8279efd49d0e7d315fddfdfe \
  dev-2c4adac3cce4860ce6686d249ef6a96bf268a2ea \
  ai.deckhouse.io/model-cache=true --overwrite

kubectl --context "$CTX" patch moduleconfig ai-models --type merge -p \
  '{"spec":{"settings":{"nodeCache":{"enabled":true,"size":"200Gi"}}}}'
```

Дождаться substrate:

```bash
kubectl --context "$CTX" -n d8-ai-models rollout status deploy/ai-models-controller --timeout=10m
kubectl --context "$CTX" get lvmvolumegroupsets.storage.deckhouse.io -o wide
kubectl --context "$CTX" get lvmvolumegroups.storage.deckhouse.io -o wide
kubectl --context "$CTX" get localstorageclasses.storage.deckhouse.io -o wide
kubectl --context "$CTX" get storageclass ai-models-node-cache -o yaml
kubectl --context "$CTX" -n d8-ai-models get pods,pvc -o wide | grep node-cache
kubectl --context "$CTX" get node "$TARGET_NODE" \
  -o jsonpath='{.metadata.labels.ai\.deckhouse\.io/node-cache-runtime-ready}{"\n"}'
```

Pass:

- `LVMVolumeGroupSet ai-models-node-cache` создан модулем;
- `LocalStorageClass ai-models-node-cache` создан модулем;
- runtime PVC для `k8s-w3-gpu.apiac.ru` `Bound`;
- runtime Pod для `k8s-w3-gpu.apiac.ru` `Running`;
- node label `ai.deckhouse.io/node-cache-runtime-ready=true`.

## 3. Проверка published ClusterModel

```bash
kubectl --context "$CTX" wait --for=jsonpath='{.status.phase}'=Ready \
  clustermodel/a30-user-bge-m3 --timeout=5m

kubectl --context "$CTX" get clustermodel a30-user-bge-m3 -o yaml
```

Фиксировать:

- `status.artifact.uri`;
- `status.artifact.digest`;
- `status.artifact.sizeBytes`;
- `status.resolved.supportedEndpointTypes`;
- `status.resolved.supportedFeatures`.

## 4. Apply ручного vLLM Deployment

В этом манифесте ai-models-specific часть — только annotation
`ai.deckhouse.io/model-refs` на metadata Deployment. Artifact URI/digest, CSI
volume, mounts и env руками не задаются: их заполнит controller.

```bash
kubectl --context "$CTX" create namespace "$NS" --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -

cat <<'YAML' | kubectl --context "$CTX" -n "$NS" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-a30-embed
  labels:
    app.kubernetes.io/name: vllm-a30-embed
    ai.deckhouse.io/live-e2e: ha-validation
  annotations:
    ai.deckhouse.io/model-refs: model=ClusterModel/a30-user-bge-m3
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: vllm-a30-embed
  template:
    metadata:
      labels:
        app.kubernetes.io/name: vllm-a30-embed
        observability.apiac.ru/service: vllm-a30-embed
        ai.deckhouse.io/live-e2e: ha-validation
    spec:
      nodeSelector:
        kubernetes.io/hostname: k8s-w3-gpu.apiac.ru
        node.deckhouse.io/group: w-gpu-mig
      tolerations:
        - key: dedicated.apiac.ru
          operator: Equal
          value: w-gpu
          effect: NoExecute
      containers:
        - name: vllm
          image: rayproject/ray-llm:2.54.0-py311-cu128
          imagePullPolicy: IfNotPresent
          command:
            - sh
            - -lc
            - >
              exec vllm serve /data/modelcache/models/model
              --runner pooling
              --convert embed
              --served-model-name a30-user-bge-m3
              --host 0.0.0.0
              --port 8000
              --dtype float16
              --gpu-memory-utilization 0.55
              --max-model-len 2048
              --max-num-seqs 8
              --max-num-batched-tokens 2048
              --enforce-eager
          env:
            - name: HF_HOME
              value: /tmp/hf
            - name: HF_HUB_DISABLE_XET
              value: "1"
            - name: HF_HUB_OFFLINE
              value: "1"
            - name: VLLM_TARGET_DEVICE
              value: cuda
          ports:
            - name: http
              containerPort: 8000
          startupProbe:
            httpGet:
              path: /health
              port: 8000
            failureThreshold: 90
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8000
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /health
              port: 8000
            initialDelaySeconds: 60
            periodSeconds: 20
          resources:
            requests:
              cpu: "1000m"
              memory: "3Gi"
              gpu.deckhouse.io/a30-mig-1g6: "1"
            limits:
              cpu: "2000m"
              memory: "6Gi"
              gpu.deckhouse.io/a30-mig-1g6: "1"
---
apiVersion: v1
kind: Service
metadata:
  name: vllm-a30-embed
  labels:
    app.kubernetes.io/name: vllm-a30-embed
    ai.deckhouse.io/live-e2e: ha-validation
spec:
  selector:
    app.kubernetes.io/name: vllm-a30-embed
  ports:
    - name: http
      port: 8000
      targetPort: 8000
YAML
```

## 5. Mutation assertions

```bash
kubectl --context "$CTX" -n "$NS" get deploy vllm-a30-embed -o yaml
kubectl --context "$CTX" -n "$NS" get pods -l app.kubernetes.io/name=vllm-a30-embed -o wide
kubectl --context "$CTX" -n "$NS" describe pod -l app.kubernetes.io/name=vllm-a30-embed
```

Pass в Deployment/PodTemplate:

- annotation `ai.deckhouse.io/model-refs` осталась user intent;
- появились resolved annotations:
  `ai.deckhouse.io/resolved-artifact-uri`,
  `ai.deckhouse.io/resolved-digest`,
  `ai.deckhouse.io/resolved-delivery-mode=SharedDirect`,
  `ai.deckhouse.io/resolved-delivery-reason=NodeCacheRuntime`;
- controller-created inline CSI volume с driver
  `node-cache.ai-models.deckhouse.io` содержит resolved artifact
  URI/digest/family attributes;
- есть mount `/data/modelcache`;
- есть env:
  `AI_MODELS_MODELS_DIR`, `AI_MODELS_MODELS`,
  `AI_MODELS_MODEL_MODEL_PATH`, `AI_MODELS_MODEL_MODEL_DIGEST`;
- нет init container `materialize-artifact`;
- нет PVC `model-cache-pvc`;
- нет projected DMCR read Secret/CA в workload namespace.

Если `resolved-delivery-mode` не `SharedDirect`, остановиться и фиксить
delivery/config. Не обходить это ручным PVC.

## 6. Runtime assertions

Дождаться Pod:

```bash
kubectl --context "$CTX" -n "$NS" rollout status deploy/vllm-a30-embed --timeout=30m
POD=$(kubectl --context "$CTX" -n "$NS" get pod -l app.kubernetes.io/name=vllm-a30-embed -o jsonpath='{.items[0].metadata.name}')
kubectl --context "$CTX" -n "$NS" logs "$POD" --since=30m
```

Проверить node-cache runtime logs:

```bash
kubectl --context "$CTX" -n d8-ai-models get pods -l ai.deckhouse.io/node-cache-runtime=managed -o wide
kubectl --context "$CTX" -n d8-ai-models logs -l ai.deckhouse.io/node-cache-runtime=managed --since=30m --all-containers
```

Pass:

- vLLM читает `/data/modelcache/models/model`;
- при `HF_HUB_OFFLINE=1` нет попытки скачать модель из Hugging Face;
- node-cache logs показывают desired artifact, materialize/prefetch, ready
  marker и CSI publish;
- runtime Pod не перезапускается при transient prefetch retry.

## 7. Functional request

```bash
kubectl --context "$CTX" -n "$NS" run curl-vllm --rm -i --restart=Never \
  --image=curlimages/curl:8.11.1 -- \
  sh -lc '
    curl -fsS http://vllm-a30-embed:8000/health &&
    curl -fsS http://vllm-a30-embed:8000/v1/models &&
    curl -fsS http://vllm-a30-embed:8000/v1/embeddings \
      -H "Content-Type: application/json" \
      -d "{\"model\":\"a30-user-bge-m3\",\"input\":[\"Deckhouse ai-models e2e\"]}" |
      head -c 1000
  '
```

Pass:

- `/health` ok;
- `/v1/models` содержит `a30-user-bge-m3`;
- `/v1/embeddings` возвращает embedding response;
- GPU allocation visible through pod resources/events and no CPU fallback.

## 8. Controlled restart drills

Выполнять только после успешного happy path.

1. Delete vLLM Pod:

   ```bash
   kubectl --context "$CTX" -n "$NS" delete pod "$POD"
   kubectl --context "$CTX" -n "$NS" rollout status deploy/vllm-a30-embed --timeout=20m
   ```

   Pass: Pod returns faster, cache reused, no full re-download.

2. Delete node-cache runtime Pod on `k8s-w3-gpu.apiac.ru`:

   ```bash
   kubectl --context "$CTX" -n d8-ai-models get pods -l ai.deckhouse.io/node-cache-runtime=managed -o wide
   kubectl --context "$CTX" -n d8-ai-models delete pod <runtime-pod-on-k8s-w3-gpu>
   kubectl --context "$CTX" -n d8-ai-models get pods -l ai.deckhouse.io/node-cache-runtime=managed -w
   ```

   Pass: runtime is recreated, ready label returns, existing workload does not
   lose a mounted read-only model path; new Pod mount retries cleanly.

3. Scale workload to zero and back:

   ```bash
   kubectl --context "$CTX" -n "$NS" scale deploy/vllm-a30-embed --replicas=0
   kubectl --context "$CTX" -n "$NS" scale deploy/vllm-a30-embed --replicas=1
   kubectl --context "$CTX" -n "$NS" rollout status deploy/vllm-a30-embed --timeout=20m
   ```

   Pass: cache remains, node-cache usage marker updated, no duplicate
   materialization.

## 9. Evidence capture

Save into `plans/active/live-e2e-ha-validation/NOTES.ru.md`:

- exact module images;
- `ModuleConfig ai-models` before/after enabling nodeCache;
- node labels and BlockDevice labels;
- substrate resources;
- vLLM Deployment before/after mutation;
- Pod events;
- node-cache runtime logs;
- vLLM logs;
- functional request output;
- restart drill timings;
- all deviations.

## 10. Cleanup

By default cleanup only the manual workload:

```bash
kubectl --context "$CTX" delete namespace "$NS" --wait=true
```

Do not disable `nodeCache` automatically after the drill. If the goal is to
return the cluster to the previous state, do it explicitly:

```bash
kubectl --context "$CTX" patch moduleconfig ai-models --type merge -p \
  '{"spec":{"settings":{"nodeCache":{"enabled":false}}}}'
```

Disabling node-cache is a separate operational decision because it removes the
runtime substrate needed by later SharedDirect tests.
