# User Flows

## 1. HuggingFace source

1. Пользователь создаёт `Model` / `ClusterModel` с
   `spec.source.type=HuggingFace`.
2. Controller валидирует source policy и создаёт publication operation.
3. Worker скачивает source snapshot, нормализует workspace и генерирует
   platform-approved `Kitfile`.
4. Worker выполняет `kit pack` + `kit push` + `kit inspect`.
5. Controller пишет:
   - `status.artifact`;
   - `status.resolved`;
   - `phase=Ready`;
   - conditions.
6. Runtime consumer later materializes the published OCI artifact through the
   runtime delivery adapter.

## 2. HTTP source

1. Пользователь создаёт object с `spec.source.type=HTTP`.
2. Controller применяет source policy:
   - URL policy;
   - archive limits;
   - optional CA/auth policy.
3. Worker скачивает archive, безопасно распаковывает, нормализует workspace.
4. Дальше flow совпадает с `HuggingFace`.

## 3. Upload source

1. Пользователь создаёт object с `spec.source.type=Upload`.
2. Controller переводит объект в `WaitForUpload`.
3. Controller создаёт upload session supplements:
   - worker pod / endpoint;
   - short-lived auth;
   - helper command/session data in `status.upload`.
4. Пользователь загружает model source.
5. Upload worker валидирует input format.
6. Дальше flow совпадает с publication path:
   normalize -> `kit pack/push/inspect` -> `status.artifact` / `status.resolved`.

## 4. Runtime consumption

1. Runtime deployment references `Model` / `ClusterModel`.
2. Controller/runtime integration resolves:
   - immutable OCI ref;
   - verification policy;
   - local unpack path.
3. `kit init` init-container pulls the artifact, verifies it, and unpacks it
   into shared PVC/volume.
4. Main runtime container starts only after successful init completion.
5. Runtime uses only the local model path.

## 5. Delete

1. Пользователь удаляет `Model` / `ClusterModel`.
2. Controller checks delete guards.
3. If allowed:
   - runtime materialization state is cleaned up;
   - OCI artifact is removed;
   - finalizer is released.
4. If not allowed:
   - object stays in deleting state with explicit condition/reason.
