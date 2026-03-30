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
from urllib.parse import quote_plus


def main() -> int:
    password = quote_plus(os.environ["AI_MODELS_DATABASE_PASSWORD"])
    user = quote_plus(os.environ["AI_MODELS_DATABASE_USER"])
    host = os.environ["AI_MODELS_DATABASE_HOST"]
    port = os.environ["AI_MODELS_DATABASE_PORT"]
    database = os.environ["AI_MODELS_DATABASE_NAME"]
    sslmode = os.environ["AI_MODELS_DATABASE_SSLMODE"]

    print(
        f"postgresql+psycopg2://{user}:{password}@{host}:{port}/{database}?sslmode={sslmode}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
