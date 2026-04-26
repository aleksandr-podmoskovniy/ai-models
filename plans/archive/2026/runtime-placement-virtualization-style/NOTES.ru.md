## Evidence 2026-04-25

### Соседний pattern

- `virtualization-controller` использует:
  - `helm_lib_node_selector (tuple . "master")`;
  - `helm_lib_tolerations (tuple . "any-node")`.
- `dvcr` использует:
  - `helm_lib_node_selector (tuple . "system")`;
  - `helm_lib_tolerations (tuple . "system")`.
- `helm_lib_node_selector "system"` не делает hard fallback на
  `node-role.kubernetes.io/control-plane`: если dedicated system nodes нет,
  selector не рендерится.

### Изменение

- `ai-models-controller` переведён на master/any-node helm-lib placement.
- `dmcr` переведён на system/system helm-lib placement.
- Локальные `ai-models.*NodeSelector` и `ai-models.*Tolerations` helpers
  удалены.
- Render fixtures дополнены минимальными `global.discovery`/`placement`
  defaults, которые нужны helm-lib helpers.
- `tools/helm-tests/validate-renders.py` теперь проверяет, что
  `Deployment/dmcr` не содержит `node-role.kubernetes.io/control-plane`.

### Render result

Baseline render:

- `Deployment/ai-models-controller`:
  - `nodeSelector: node-role.kubernetes.io/control-plane=""`;
  - tolerations из `any-node`.
- `Deployment/dmcr`:
  - `nodeSelector` отсутствует при `system=0`;
  - tolerations из `system`.

Это соответствует целевому смыслу: controller остаётся control-plane
компонентом как в `virtualization-controller`, а тяжёлый internal registry
runtime больше не прибивается к master/control-plane при отсутствии system
nodes.

### Проверки

- `make helm-template` — успешно.
- `make kubeconform` — успешно.
- `make verify` — успешно.
- `git diff --check` — успешно.
