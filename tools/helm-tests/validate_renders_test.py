#!/usr/bin/env python3

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import hashlib
import importlib.util
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("validate-renders.py")
SPEC = importlib.util.spec_from_file_location("validate_renders", SCRIPT_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class ValidateRendersTest(unittest.TestCase):
    @staticmethod
    def _controller_contract_docs() -> str:
        return """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-ca
type: kubernetes.io/tls
stringData:
  tls.crt: cert
  tls.key: key
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ai-models-ca
data:
  ca-bundle: |
    cert
"""

    @staticmethod
    def _htpasswd_entry(username: str, password: str) -> str:
        htpasswd_bin = shutil.which("htpasswd")
        if not htpasswd_bin:
            return f"{username}:placeholder"

        result = subprocess.run(
            [htpasswd_bin, "-nbB", username, password],
            capture_output=True,
            text=True,
            check=True,
        )
        return result.stdout.strip()

    @staticmethod
    def _write_human_rbac_sources(root: Path, **overrides: str) -> None:
        defaults = {
            "templates/user-authz-cluster-roles.yaml": """---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: User
  name: d8:user-authz:ai-models:user
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models", "clustermodels"]
    verbs: ["get", "list", "watch"]
---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: PrivilegedUser
  name: d8:user-authz:ai-models:privileged-user
  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}
rules: []
---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Editor
  name: d8:user-authz:ai-models:editor
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models"]
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: Admin
  name: d8:user-authz:ai-models:admin
  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}
rules: []
---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: ClusterEditor
  name: d8:user-authz:ai-models:cluster-editor
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["clustermodels"]
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
---
metadata:
  annotations:
    user-authz.deckhouse.io/access-level: ClusterAdmin
  name: d8:user-authz:ai-models:cluster-admin
  {{- include "helm_lib_module_labels" (list .) | nindent 2 }}
rules: []
""",
            "templates/rbacv2/use/view.yaml": """metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/kind: use
  name: d8:use:capability:module:ai-models:view
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models"]
    verbs: ["get", "list", "watch"]
""",
            "templates/rbacv2/use/edit.yaml": """metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    rbac.deckhouse.io/kind: use
  name: d8:use:capability:module:ai-models:edit
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models"]
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
""",
            "templates/rbacv2/manage/view.yaml": """metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: viewer
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: module
  name: d8:manage:permission:module:ai-models:view
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models", "clustermodels"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    resourceNames: ["ai-models"]
    verbs: ["get", "list", "watch"]
""",
            "templates/rbacv2/manage/edit.yaml": """metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: module
  name: d8:manage:permission:module:ai-models:edit
rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models", "clustermodels"]
    verbs: ["create", "update", "patch", "delete", "deletecollection"]
  - apiGroups: ["deckhouse.io"]
    resources: ["moduleconfigs"]
    resourceNames: ["ai-models"]
    verbs: ["create", "update", "patch", "delete"]
""",
        }

        for relative_path, content in defaults.items():
            target = root / relative_path
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(overrides.get(relative_path, content), encoding="utf-8")

    def test_validate_human_rbac_sources_accepts_target_contract(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            self._write_human_rbac_sources(root)
            errors = MODULE._validate_human_rbac_sources(root)

        self.assertEqual(errors, [])

    def test_validate_human_rbac_sources_rejects_forbidden_access(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            self._write_human_rbac_sources(
                root,
                **{
                    "templates/rbacv2/use/edit.yaml": """rules:
  - apiGroups: ["ai.deckhouse.io"]
    resources: ["models", "clustermodels", "models/status"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get"]
""",
                },
            )
            errors = MODULE._validate_human_rbac_sources(root)

        self.assertIn(
            "templates/rbacv2/use/edit.yaml: human-facing RBAC must not grant models/status",
            errors,
        )
        self.assertIn(
            "templates/rbacv2/use/edit.yaml: human-facing RBAC must not grant secrets",
            errors,
        )
        self.assertIn(
            "templates/rbacv2/use: use roles must not grant cluster-scoped clustermodels",
            errors,
        )
        self.assertIn(
            "templates/rbacv2/use/edit.yaml: edit role must be write-only delta",
            errors,
        )

    @staticmethod
    def _write_metrics_proxy_rbac_sources(
        root: Path, dmcr_content: str | None = None
    ) -> None:
        controller = root / "templates/controller/rbac.yaml"
        controller.parent.mkdir(parents=True, exist_ok=True)
        controller.write_text(
            """---
kind: ClusterRole
metadata:
  name: {{ include "ai-models.controllerName" . }}-metrics-auth
rules:
  - apiGroups: ["authentication.k8s.io"]
    resources: ["tokenreviews"]
    verbs: ["create"]
  - apiGroups: ["authorization.k8s.io"]
    resources: ["subjectaccessreviews"]
    verbs: ["create"]
---
kind: ClusterRoleBinding
metadata:
  name: {{ include "ai-models.controllerName" . }}-metrics-auth
roleRef:
  kind: ClusterRole
subjects:
  - kind: ServiceAccount
    name: {{ include "ai-models.controllerServiceAccountName" . }}
""",
            encoding="utf-8",
        )

        dmcr = root / "templates/dmcr/rbac.yaml"
        dmcr.parent.mkdir(parents=True, exist_ok=True)
        dmcr.write_text(
            dmcr_content
            or """---
kind: ClusterRole
metadata:
  name: {{ include "ai-models.dmcrName" . }}-metrics-auth
rules:
  - apiGroups: ["authentication.k8s.io"]
    resources: ["tokenreviews"]
    verbs: ["create"]
  - apiGroups: ["authorization.k8s.io"]
    resources: ["subjectaccessreviews"]
    verbs: ["create"]
---
kind: ClusterRoleBinding
metadata:
  name: {{ include "ai-models.dmcrName" . }}-metrics-auth
roleRef:
  kind: ClusterRole
subjects:
  - kind: ServiceAccount
    name: {{ include "ai-models.dmcrServiceAccountName" . }}
""",
            encoding="utf-8",
        )

        rbac_to_us = root / "templates/rbac-to-us.yaml"
        rbac_to_us.parent.mkdir(parents=True, exist_ok=True)
        rbac_to_us.write_text(
            """---
kind: Role
metadata:
  name: access-to-ai-models-prometheus-metrics
rules:
  - apiGroups: ["apps"]
    resources: ["deployments/prometheus-metrics"]
    resourceNames:
      - {{ include "ai-models.controllerName" . }}
      - {{ include "ai-models.dmcrName" . }}
    verbs: ["get"]
---
kind: RoleBinding
metadata:
  name: access-to-ai-models-prometheus-metrics
subjects:
  - kind: User
    name: d8-monitoring:scraper
  - kind: ServiceAccount
    name: prometheus
    namespace: d8-monitoring
""",
            encoding="utf-8",
        )

    def test_validate_metrics_proxy_rbac_accepts_authn_authz_rules(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            self._write_metrics_proxy_rbac_sources(root)
            errors = MODULE._validate_metrics_proxy_rbac_sources(root)

        self.assertEqual(errors, [])

    def test_validate_metrics_proxy_rbac_rejects_missing_dmcr_tokenreview(self) -> None:
        with tempfile.TemporaryDirectory() as tmpdir:
            root = Path(tmpdir)
            self._write_metrics_proxy_rbac_sources(
                root,
                dmcr_content="""---
kind: ClusterRole
metadata:
  name: {{ include "ai-models.dmcrName" . }}-metrics-auth
rules:
  - apiGroups: ["authorization.k8s.io"]
    resources: ["subjectaccessreviews"]
    verbs: ["create"]
---
kind: ClusterRoleBinding
metadata:
  name: {{ include "ai-models.dmcrName" . }}-metrics-auth
roleRef:
  kind: ClusterRole
subjects:
  - kind: ServiceAccount
    name: {{ include "ai-models.dmcrServiceAccountName" . }}
""",
            )
            errors = MODULE._validate_metrics_proxy_rbac_sources(root)

        self.assertIn(
            'templates/dmcr/rbac.yaml: missing resources: ["tokenreviews"]',
            errors,
        )

    def test_parse_secret_documents_reads_string_data(self) -> None:
        content = """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: auth/bootstrap
stringData:
  write.password: "writer"
  write.htpasswd: |
    ai-models:$2y$example
  write.htpasswd.checksum: "abc123"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ignore-me
"""

        documents = MODULE._parse_secret_documents(content)
        self.assertEqual(len(documents), 1)
        self.assertEqual(documents[0]["metadata"]["name"], "ai-models-dmcr-auth")
        self.assertEqual(
            documents[0]["metadata"]["annotations"][
                "ai.deckhouse.io/dmcr-pod-secret-checksum"
            ],
            "auth/bootstrap",
        )
        self.assertEqual(documents[0]["stringData"]["write.password"], "writer")
        self.assertEqual(
            documents[0]["stringData"]["write.htpasswd"], "ai-models:$2y$example"
        )

    def test_validate_render_accepts_dmcr_auth_without_pyyaml(self) -> None:
        write_password = "writer-password"
        read_password = "reader-password"
        write_checksum = hashlib.sha256(write_password.encode("utf-8")).hexdigest()
        read_checksum = hashlib.sha256(read_password.encode("utf-8")).hexdigest()
        write_htpasswd = self._htpasswd_entry("ai-models", write_password)
        read_htpasswd = self._htpasswd_entry("ai-models-reader", read_password)
        content = """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth
stringData:
  write.password: "{write_password}"
  read.password: "{read_password}"
  write.htpasswd: |
    {write_htpasswd}
  write.htpasswd.checksum: "{write_checksum}"
  read.htpasswd: |
    {read_htpasswd}
  read.htpasswd.checksum: "{read_checksum}"
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth-write
stringData:
  username: "ai-models"
  password: "{write_password}"
  .dockerconfigjson: |
    {{"auths":{{"dmcr.d8-ai-models.svc.cluster.local":{{"username":"ai-models","password":"{write_password}","auth":"YWktbW9kZWxzOndyaXRlci1wYXNzd29yZA=="}}}}}}
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth-read
stringData:
  username: "ai-models-reader"
  password: "{read_password}"
  .dockerconfigjson: |
    {{"auths":{{"dmcr.d8-ai-models.svc.cluster.local":{{"username":"ai-models-reader","password":"{read_password}","auth":"YWktbW9kZWxzLXJlYWRlcjpyZWFkZXItcGFzc3dvcmQ="}}}}}}
""".format(
            write_password=write_password,
            read_password=read_password,
            write_checksum=write_checksum,
            read_checksum=read_checksum,
            write_htpasswd=write_htpasswd,
            read_htpasswd=read_htpasswd,
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(errors, [])

    def test_validate_render_accepts_dmcr_restart_checksum_contract(self) -> None:
        expected_checksum = hashlib.sha256(
            b"auth/bootstrap\ntls/bootstrap"
        ).hexdigest()
        content = """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-tls
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: tls/bootstrap
type: kubernetes.io/tls
stringData:
  tls.crt: cert
  tls.key: key
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: auth/bootstrap
stringData:
  write.password: writer
  read.password: reader
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth-write
stringData:
  password: writer
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dmcr
spec:
  template:
    metadata:
      annotations:
        checksum/secret: "{expected_checksum}"
""".format(
            expected_checksum=expected_checksum
        )

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(errors, [])

    def test_validate_render_rejects_dmcr_restart_checksum_drift(self) -> None:
        content = """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-tls
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: tls/bootstrap
type: kubernetes.io/tls
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: auth/bootstrap
---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-ca
  annotations:
    ai.deckhouse.io/dmcr-pod-secret-checksum: forbidden
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dmcr
spec:
  template:
    metadata:
      annotations:
        checksum/secret: wrong
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: Deployment/dmcr checksum/secret does not match DMCR runtime Secret restart annotations",
                "helm-template-test.yaml: ai-models-dmcr-ca must not participate in Deployment/dmcr restart checksum",
            ],
        )

    def test_validate_render_rejects_too_long_port_name(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dmcr
spec:
  template:
    spec:
      containers:
        - name: dmcr-direct-upload
          ports:
            - name: https-direct-upload
              containerPort: 5002
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: port name 'https-direct-upload' exceeds Kubernetes 15-character limit"
            ],
        )

    def test_validate_render_rejects_dmcr_role_without_secret_delete(self) -> None:
        content = """---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: dmcr
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "update", "patch"]
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: Role/dmcr must grant delete on secrets for dmcr garbage-collection request cleanup"
            ],
        )

    def test_validate_render_rejects_missing_stable_node_cache_runtime_plane(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-models-controller
spec:
  template:
    spec:
      containers:
        - name: controller
          env:
            - name: AI_MODELS_OCI_USERNAME
              value: ai-models-reader
            - name: AI_MODELS_OCI_PASSWORD
              value: reader
          args:
            - --node-cache-enabled=true
""" + self._controller_contract_docs()

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: controller render must pass --node-cache-node-selector-json for SDS-backed node-cache substrate",
                "helm-template-test.yaml: controller render must pass --node-cache-block-device-selector-json for SDS-backed node-cache substrate",
                "helm-template-test.yaml: controller render must pass --node-cache-shared-volume-size for the stable node-cache runtime PVC contract",
                "helm-template-test.yaml: controller render must pass --node-cache-csi-registrar-image for kubelet-facing node-cache CSI registration",
                "helm-template-test.yaml: controller render must pass --node-cache-runtime-image for the dedicated node-cache runtime image",
                "helm-template-test.yaml: node-cache-enabled render must include ServiceAccount/ai-models-node-cache-runtime",
                "helm-template-test.yaml: node-cache-enabled render must include Role/ai-models-node-cache-runtime",
                "helm-template-test.yaml: node-cache-enabled render must include RoleBinding/ai-models-node-cache-runtime",
                "helm-template-test.yaml: node-cache runtime RBAC must grant read-only get/list on pods for CSI publish authorization",
                "helm-template-test.yaml: node-cache-enabled render must include CSIDriver/node-cache.ai-models.deckhouse.io",
            ],
        )

    def test_validate_render_accepts_stable_node_cache_runtime_plane(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-models-controller
spec:
  template:
    spec:
      containers:
        - name: controller
          env:
            - name: AI_MODELS_OCI_USERNAME
              value: ai-models-reader
            - name: AI_MODELS_OCI_PASSWORD
              value: reader
          args:
            - --node-cache-enabled=true
            - --node-cache-runtime-image=registry.example.test/node-cache-runtime@sha256:2222
            - --node-cache-csi-registrar-image=registry.example.test/csi-node-driver-registrar@sha256:1111
            - --node-cache-shared-volume-size=64Gi
            - --node-cache-node-selector-json={"ai.deckhouse.io/node-cache":"true"}
            - --node-cache-block-device-selector-json={"ai.deckhouse.io/model-cache":"true"}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ai-models-node-cache-runtime
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: node-cache.ai-models.deckhouse.io
""" + self._controller_contract_docs()

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(errors, [])

    def test_validate_render_rejects_empty_node_cache_selectors(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-models-controller
spec:
  template:
    spec:
      containers:
        - name: controller
          env:
            - name: AI_MODELS_OCI_USERNAME
              value: ai-models-reader
            - name: AI_MODELS_OCI_PASSWORD
              value: reader
          args:
            - --node-cache-enabled=true
            - --node-cache-runtime-image=registry.example.test/node-cache-runtime@sha256:2222
            - --node-cache-csi-registrar-image=registry.example.test/csi-node-driver-registrar@sha256:1111
            - --node-cache-shared-volume-size=64Gi
            - --node-cache-node-selector-json={}
            - --node-cache-block-device-selector-json={}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ai-models-node-cache-runtime
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: node-cache.ai-models.deckhouse.io
""" + self._controller_contract_docs()

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: controller render must not pass an empty --node-cache-node-selector-json when node-cache is enabled",
                "helm-template-test.yaml: controller render must not pass an empty --node-cache-block-device-selector-json when node-cache is enabled",
            ],
        )

    def test_validate_render_rejects_quoted_node_cache_json_flags(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-models-controller
spec:
  template:
    spec:
      containers:
        - name: controller
          env:
            - name: AI_MODELS_OCI_USERNAME
              value: ai-models-reader
            - name: AI_MODELS_OCI_PASSWORD
              value: reader
          args:
            - --node-cache-enabled=false
            - --node-cache-node-selector-json="{}"
            - --node-cache-block-device-selector-json="{\\"role\\":\\"gpu\\"}"
""" + self._controller_contract_docs()

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: controller render must not wrap --node-cache-node-selector-json value in extra quotes inside the argument",
                "helm-template-test.yaml: controller render must not wrap --node-cache-block-device-selector-json value in extra quotes inside the argument",
            ],
        )

    def test_validate_render_rejects_legacy_node_cache_runtime_daemonset(self) -> None:
        content = """---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-models-controller
spec:
  template:
    spec:
      containers:
        - name: controller
          env:
            - name: AI_MODELS_OCI_USERNAME
              value: ai-models-reader
            - name: AI_MODELS_OCI_PASSWORD
              value: reader
          args:
            - --node-cache-enabled=true
            - --node-cache-runtime-image=registry.example.test/node-cache-runtime@sha256:2222
            - --node-cache-csi-registrar-image=registry.example.test/csi-node-driver-registrar@sha256:1111
            - --node-cache-shared-volume-size=64Gi
            - --node-cache-node-selector-json={"ai.deckhouse.io/node-cache":"true"}
            - --node-cache-block-device-selector-json={"ai.deckhouse.io/model-cache":"true"}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ai-models-node-cache-runtime
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: node-cache.ai-models.deckhouse.io
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ai-models-node-cache-runtime
""" + self._controller_contract_docs()

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: node-cache-enabled render must not keep legacy DaemonSet/ai-models-node-cache-runtime after stable per-node runtime plane rollout",
            ],
        )

    def test_validate_render_accepts_fail_closed_workload_delivery_webhook(self) -> None:
        content = """---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: ai-models-workload-delivery
webhooks:
  - name: workloaddelivery.ai-models.deckhouse.io
    failurePolicy: Fail
    clientConfig:
      caBundle: Y2E=
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(errors, [])

    def test_validate_render_rejects_fail_open_workload_delivery_webhook(self) -> None:
        content = """---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: ai-models-workload-delivery
webhooks:
  - name: workloaddelivery.ai-models.deckhouse.io
    failurePolicy: Ignore
    clientConfig:
      caBundle: ""
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: workload delivery webhook must fail closed for annotated workloads",
                "helm-template-test.yaml: workload delivery webhook must render a non-empty caBundle",
            ],
        )


if __name__ == "__main__":
    unittest.main()
