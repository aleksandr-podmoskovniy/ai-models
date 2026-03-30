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

import os
import sys
from configparser import ConfigParser
from pathlib import Path


def main() -> int:
    output_path = Path(sys.argv[1])
    database_uri = sys.stdin.read().strip()

    config = ConfigParser()
    config["mlflow"] = {
        "default_permission": "NO_PERMISSIONS",
        "database_uri": database_uri,
        "admin_username": os.environ["AI_MODELS_AUTH_ADMIN_USERNAME"],
        "admin_password": os.environ["AI_MODELS_AUTH_ADMIN_PASSWORD"],
        "authorization_function": "mlflow.server.auth:authenticate_request_basic_auth",
        "grant_default_workspace_access": "false",
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    with output_path.open("w", encoding="utf-8") as output:
        config.write(output)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
