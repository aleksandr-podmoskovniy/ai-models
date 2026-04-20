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

    def test_parse_secret_documents_reads_string_data(self) -> None:
        content = """---
apiVersion: v1
kind: Secret
metadata:
  name: ai-models-dmcr-auth
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
          args:
            - --node-cache-enabled=true
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(
            errors,
            [
                "helm-template-test.yaml: controller render must pass --node-cache-shared-volume-size for the stable node-cache runtime PVC contract",
                "helm-template-test.yaml: node-cache-enabled render must include ServiceAccount/ai-models-node-cache-runtime",
                "helm-template-test.yaml: node-cache-enabled render must include Role/ai-models-node-cache-runtime",
                "helm-template-test.yaml: node-cache-enabled render must include RoleBinding/ai-models-node-cache-runtime",
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
          args:
            - --node-cache-enabled=true
            - --node-cache-shared-volume-size=64Gi
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
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ai-models-node-cache-runtime
"""

        with tempfile.TemporaryDirectory() as tmpdir:
            render_path = Path(tmpdir) / "helm-template-test.yaml"
            render_path.write_text(content, encoding="utf-8")
            errors = MODULE.validate_render(render_path)

        self.assertEqual(errors, [])

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
          args:
            - --node-cache-enabled=false
            - --node-cache-node-selector-json="{}"
            - --node-cache-block-device-selector-json="{\\"role\\":\\"gpu\\"}"
"""

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
          args:
            - --node-cache-enabled=true
            - --node-cache-shared-volume-size=64Gi
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
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ai-models-node-cache-runtime
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ai-models-node-cache-runtime
"""

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


if __name__ == "__main__":
    unittest.main()
