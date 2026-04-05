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

import importlib.util
import io
from pathlib import Path
import sys
import tarfile
import tempfile
import types
import unittest
import zipfile


def load_source_publish_module():
    script_path = Path(__file__).with_name("ai-models-backend-source-publish.py")

    fake_hf = types.ModuleType("huggingface_hub")

    class _FakeHfApi:
        def model_info(self, *args, **kwargs):
            raise RuntimeError("not used in archive safety tests")

    fake_hf.HfApi = _FakeHfApi
    fake_hf.snapshot_download = lambda *args, **kwargs: None
    sys.modules.setdefault("huggingface_hub", fake_hf)

    fake_runtime = types.ModuleType("ai_models_backend_runtime")
    fake_runtime.env = lambda key, default="": default
    sys.modules.setdefault("ai_models_backend_runtime", fake_runtime)

    spec = importlib.util.spec_from_file_location("ai_models_backend_source_publish", script_path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


SOURCE_PUBLISH = load_source_publish_module()


class ArchiveSafetyTests(unittest.TestCase):
    def test_safe_extract_tar_rejects_symbolic_links(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            archive_path = Path(temp_dir) / "model.tar"
            destination = Path(temp_dir) / "out"

            with tarfile.open(archive_path, "w") as archive:
                link = tarfile.TarInfo("checkpoint-link")
                link.type = tarfile.SYMTYPE
                link.linkname = "../escape"
                archive.addfile(link)

            with self.assertRaisesRegex(RuntimeError, "symbolic link"):
                SOURCE_PUBLISH.safe_extract_tar(archive_path, destination)

    def test_safe_extract_tar_rejects_hard_links(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            archive_path = Path(temp_dir) / "model.tar"
            destination = Path(temp_dir) / "out"

            with tarfile.open(archive_path, "w") as archive:
                regular = tarfile.TarInfo("checkpoint/config.json")
                regular_bytes = b"{}"
                regular.size = len(regular_bytes)
                archive.addfile(regular, io.BytesIO(regular_bytes))

                link = tarfile.TarInfo("checkpoint-link")
                link.type = tarfile.LNKTYPE
                link.linkname = "checkpoint/config.json"
                archive.addfile(link)

            with self.assertRaisesRegex(RuntimeError, "hard link"):
                SOURCE_PUBLISH.safe_extract_tar(archive_path, destination)

    def test_safe_extract_zip_rejects_path_traversal(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            archive_path = Path(temp_dir) / "model.zip"
            destination = Path(temp_dir) / "out"

            with zipfile.ZipFile(archive_path, "w") as archive:
                archive.writestr("../escape.txt", "boom")

            with self.assertRaisesRegex(RuntimeError, "outside of destination"):
                SOURCE_PUBLISH.safe_extract_zip(archive_path, destination)

    def test_safe_extract_zip_rejects_symbolic_links(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            archive_path = Path(temp_dir) / "model.zip"
            destination = Path(temp_dir) / "out"

            with zipfile.ZipFile(archive_path, "w") as archive:
                symlink = zipfile.ZipInfo("checkpoint-link")
                symlink.create_system = 3
                symlink.external_attr = 0o120777 << 16
                archive.writestr(symlink, "../escape")

            with self.assertRaisesRegex(RuntimeError, "symbolic link"):
                SOURCE_PUBLISH.safe_extract_zip(archive_path, destination)

    def test_unpack_http_archive_extracts_regular_checkpoint_archive(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            archive_path = Path(temp_dir) / "model.tar.gz"
            destination = Path(temp_dir) / "out"

            with tarfile.open(archive_path, "w:gz") as archive:
                config = tarfile.TarInfo("checkpoint/config.json")
                config_bytes = b'{"model_type":"llama"}'
                config.size = len(config_bytes)
                archive.addfile(config, io.BytesIO(config_bytes))

            extracted_root = SOURCE_PUBLISH.unpack_http_archive(archive_path, destination)

            self.assertEqual(extracted_root, destination / "checkpoint")
            self.assertTrue((extracted_root / "config.json").is_file())


class OCIHelpersTests(unittest.TestCase):
    def test_registry_from_oci_reference_extracts_explicit_registry(self):
        registry = SOURCE_PUBLISH.registry_from_oci_reference(
            "registry.example.com/ai-models/catalog/namespaced/team-a/model:published"
        )

        self.assertEqual(registry, "registry.example.com")

    def test_registry_from_oci_reference_rejects_bare_repository(self):
        with self.assertRaisesRegex(RuntimeError, "explicit OCI registry host"):
            SOURCE_PUBLISH.registry_from_oci_reference("ai-models/catalog/model:published")

    def test_immutable_oci_reference_rewrites_tag_to_digest(self):
        reference = SOURCE_PUBLISH.immutable_oci_reference(
            "registry.example.com/ai-models/catalog/model:published",
            "sha256:deadbeef",
        )

        self.assertEqual(reference, "registry.example.com/ai-models/catalog/model@sha256:deadbeef")

    def test_build_backend_result_uses_oci_artifact_and_digest_reference(self):
        result = SOURCE_PUBLISH.build_backend_result(
            hf_context={"repo_id": "deepseek-ai/DeepSeek-R1", "resolved_revision": "abc123"},
            task="text-generation",
            config_summary={"model_type": "deepseek", "architectures": "DeepseekForCausalLM"},
            artifact_uri="registry.example.com/ai-models/catalog/model:published",
            artifact_digest="sha256:deadbeef",
            artifact_media_type="application/vnd.cncf.model.manifest.v1+json",
            artifact_size_bytes=123,
        )

        self.assertEqual(result["artifact"]["kind"], "OCI")
        self.assertEqual(result["artifact"]["digest"], "sha256:deadbeef")
        self.assertEqual(result["cleanupHandle"]["backend"]["reference"], "registry.example.com/ai-models/catalog/model@sha256:deadbeef")


class HTTPAuthTests(unittest.TestCase):
    def test_http_auth_headers_from_dir_supports_authorization_file(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            auth_dir = Path(temp_dir)
            (auth_dir / "authorization").write_text("Bearer abc", encoding="utf-8")

            headers = SOURCE_PUBLISH.http_auth_headers_from_dir(str(auth_dir))

            self.assertEqual(headers, {"Authorization": "Bearer abc"})

    def test_http_auth_headers_from_dir_supports_basic_pair(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            auth_dir = Path(temp_dir)
            (auth_dir / "username").write_text("alice", encoding="utf-8")
            (auth_dir / "password").write_text("secret", encoding="utf-8")

            headers = SOURCE_PUBLISH.http_auth_headers_from_dir(str(auth_dir))

            self.assertEqual(headers, {"Authorization": "Basic YWxpY2U6c2VjcmV0"})

    def test_http_auth_headers_from_dir_rejects_missing_directory(self):
        with self.assertRaisesRegex(RuntimeError, "does not exist"):
            SOURCE_PUBLISH.http_auth_headers_from_dir("/definitely/missing/auth-dir")


if __name__ == "__main__":
    unittest.main()
