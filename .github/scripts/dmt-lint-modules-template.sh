#!/usr/bin/env bash

# Copyright 2026 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Lints the modules-template repository with a locally built dmt binary.
# This verifies that changes in this repository do not break (or unexpectedly
# change) linting of the reference module template.
#
# Required environment variables:
#   SRC_DIR  - directory where modules-template is (or will be) checked out
# Optional:
#   DMT_BIN  - dmt binary to use (default: "dmt" from PATH)

set -euo pipefail

DMT_BIN="${DMT_BIN:-dmt}"
SRC_DIR="${SRC_DIR:?SRC_DIR (directory for modules-template checkout) is required}"

echo "Linting with: ${DMT_BIN}"
"${DMT_BIN}" --version || true

echo "Running: ${DMT_BIN} lint -l INFO ${SRC_DIR}"
"${DMT_BIN}" lint -l INFO "${SRC_DIR}"
