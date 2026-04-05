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

from __future__ import annotations

import argparse
import http.server
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from typing import Any


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Serve a controller-owned upload session and publish the uploaded archive into the current backend artifact plane."
    )
    parser.add_argument("--artifact-uri", required=True)
    parser.add_argument("--result-configmap-name", required=True)
    parser.add_argument("--result-configmap-namespace", required=True)
    parser.add_argument("--expected-format", required=True)
    parser.add_argument("--expected-size-bytes", type=int, default=0)
    parser.add_argument("--task", default="")
    parser.add_argument("--listen-port", type=int, default=8444)
    parser.add_argument("--upload-token", default=os.getenv("AI_MODELS_UPLOAD_TOKEN", ""))
    return parser


def read_exact(stream: Any, length: int, destination: Path) -> int:
    remaining = length
    written = 0
    with destination.open("wb") as handle:
        while remaining > 0:
            chunk = stream.read(min(1024 * 1024, remaining))
            if not chunk:
                break
            handle.write(chunk)
            written += len(chunk)
            remaining -= len(chunk)
    return written


class UploadServer(http.server.HTTPServer):
    def __init__(self, server_address: tuple[str, int], request_handler_class: type[http.server.BaseHTTPRequestHandler], args: argparse.Namespace, upload_dir: Path):
        super().__init__(server_address, request_handler_class)
        self.args = args
        self.upload_dir = upload_dir
        self.stop_requested = False
        self.exit_code = 0


class UploadHandler(http.server.BaseHTTPRequestHandler):
    server: UploadServer

    def do_PUT(self) -> None:  # noqa: N802
        if self.path != "/upload":
            self.send_error(404, "unknown upload path")
            return

        expected_token = self.server.args.upload_token.strip()
        if not expected_token:
            self.send_error(500, "upload token is not configured")
            self.server.exit_code = 1
            self.server.stop_requested = True
            return

        auth_header = self.headers.get("Authorization", "").strip()
        if auth_header != f"Bearer {expected_token}":
            self.send_error(401, "invalid upload token")
            return

        content_length = self.headers.get("Content-Length", "").strip()
        if not content_length:
            self.send_error(411, "Content-Length header is required")
            return

        try:
            length = int(content_length)
        except ValueError:
            self.send_error(400, "invalid Content-Length header")
            return

        if length <= 0:
            self.send_error(400, "upload body must not be empty")
            return

        if self.server.args.expected_size_bytes > 0 and length != self.server.args.expected_size_bytes:
            self.send_error(400, "uploaded payload size does not match expected-size-bytes")
            return

        upload_path = self.server.upload_dir / "upload.bin"
        written = read_exact(self.rfile, length, upload_path)
        if written != length:
            self.send_error(400, "unexpected end of upload stream")
            return

        cmd = [
            "ai-models-backend-source-publish",
            "--source-type",
            "Upload",
            "--artifact-uri",
            self.server.args.artifact_uri,
            "--result-configmap-name",
            self.server.args.result_configmap_name,
            "--result-configmap-namespace",
            self.server.args.result_configmap_namespace,
            "--upload-path",
            str(upload_path),
            "--upload-format",
            self.server.args.expected_format,
        ]
        if self.server.args.task:
            cmd.extend(["--task", self.server.args.task])

        completed = subprocess.run(cmd, check=False)
        if completed.returncode != 0:
            self.send_error(500, "upload processing failed")
            self.server.exit_code = completed.returncode
            self.server.stop_requested = True
            return

        self.send_response(201)
        self.end_headers()
        self.wfile.write(b"upload accepted\n")
        self.server.stop_requested = True

    def do_GET(self) -> None:  # noqa: N802
        if self.path == "/healthz":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"ok\n")
            return
        self.send_error(404, "unknown path")

    def log_message(self, format: str, *args: Any) -> None:  # noqa: A003
        print(format % args, file=sys.stderr)


def main() -> int:
    args = build_parser().parse_args()
    if not args.task:
        print("Error: task is required for upload session publication", file=sys.stderr)
        return 1
    if not args.upload_token:
        print("Error: upload-token is required", file=sys.stderr)
        return 1

    upload_dir = Path(tempfile.mkdtemp(prefix="ai-model-upload-session-"))
    server = UploadServer(("", args.listen_port), UploadHandler, args, upload_dir)
    try:
        while not server.stop_requested:
            server.handle_request()
        return server.exit_code
    finally:
        shutil.rmtree(upload_dir, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())
