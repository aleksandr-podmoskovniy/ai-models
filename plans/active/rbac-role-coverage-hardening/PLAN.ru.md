### 1. Current phase

Этап 1/2 runtime baseline: публичный catalog API уже живой, RBAC должен быть
строго согласован с Deckhouse access-level model и rbacv2 aggregation.

### 2. Orchestration

`solo`

Причина: slice ограничен Helm RBAC templates и machine-checkable validation,
без изменения API/controller runtime semantics. Паттерны сверяются напрямую по
локальным `deckhouse` и `virtualization` репозиториям.

### 3. Slices

#### Slice 1. RBAC template parity

Цель:

- привести legacy `user-authz` к delta-fragment форме как в virtualization;
- оставить `rbacv2/use` только для namespaced `Model`;
- оставить `rbacv2/manage` для cluster-persona управления `Model`,
  `ClusterModel` и `ModuleConfig`.

Файлы:

- `templates/user-authz-cluster-roles.yaml`
- `templates/rbacv2/use/*.yaml`
- `templates/rbacv2/manage/*.yaml`

Проверки:

- manual compare with:
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/virtualization/templates/user-authz-cluster-roles.yaml`
  - `/Users/myskat_90/flant/aleksandr-podmoskovniy/deckhouse/modules/140-user-authz/templates/rbacv2/global/use/roles/kubernetes/*.yaml`

#### Slice 2. RBAC guardrail validation

Цель:

- добавить render/source validation, которая не даст вернуть forbidden
  human-facing permissions.

Файлы:

- `tools/helm-tests/validate-renders.py`
- `tools/helm-tests/validate_renders_test.py`

Проверки:

- `python3 tools/helm-tests/validate_renders_test.py`

#### Slice 3. Docs and final validation

Цель:

- зафиксировать RBAC coverage в docs;
- прогнать render/kubeconform и repo-level checks.

Файлы:

- `docs/CONFIGURATION.md`
- `docs/CONFIGURATION.ru.md`

Проверки:

- `make helm-template`
- `make kubeconform`
- `make verify`

### 4. Rollback point

После Slice 1 можно откатить только RBAC templates без затрагивания runtime и
контроллеров.

### 5. Final validation

- `python3 tools/helm-tests/validate_renders_test.py`
- `make helm-template`
- `make kubeconform`
- `make verify`

### 6. Status

- Slice 1 implemented:
  - legacy `user-authz` now follows delta-fragment semantics:
    `User` reads `Model` / `ClusterModel`, `Editor` writes namespaced `Model`,
    `ClusterEditor` writes cluster-wide `ClusterModel`;
  - `PrivilegedUser`, `Admin`, and `ClusterAdmin` are explicitly present but
    empty for ai-models-specific extra verbs;
  - `rbacv2/use` stays namespaced `Model` only;
  - `rbacv2/manage` stays cluster-persona access for `Model`, `ClusterModel`,
    and `ModuleConfig ai-models`.
- Slice 2 implemented:
  - `validate-renders.py` now checks human-facing RBAC source templates for
    forbidden `status`, `finalizers`, Secret, pod log/exec/attach/port-forward
    and internal runtime resource grants.
- Slice 3 partially validated:
  - `python3 tools/helm-tests/validate_renders_test.py` passed;
  - `make helm-template` passed;
  - `make kubeconform` passed;
  - `make verify` passed;
  - `git diff --check && git diff --cached --check` passed.

### 7. Review notes

- No critical findings in the RBAC slice.
- Service-account RBAC still contains controller/runtime permissions for
  `status`, `finalizers`, Secrets and internal operations, but those templates
  are not labeled or aggregated into human-facing roles.
- `ClusterAdmin` remains an explicit empty ai-models delta until the module has
  service cluster-wide objects that should be granted at that level.
