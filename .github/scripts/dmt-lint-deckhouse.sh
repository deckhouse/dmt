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

# Lints the deckhouse repository the same way deckhouse/tools/dmt-lint.sh does,
# but with a locally built dmt binary instead of a released one. This verifies
# that changes in this repository do not break (or unexpectedly change) linting
# of the real deckhouse codebase.
#
# Required environment variables:
#   SRC_DIR  - path to a checkout of the deckhouse repository
# Optional:
#   DMT_BIN  - dmt binary to use (default: "dmt" from PATH)
#   WORK_DIR - scratch directory for the prepared structure (default: ${SRC_DIR}-prepared)

set -euo pipefail

DMT_BIN="${DMT_BIN:-dmt}"
SRC_DIR="${SRC_DIR:?SRC_DIR (path to a deckhouse checkout) is required}"
WORK_DIR="${WORK_DIR:-${SRC_DIR}-prepared}"

# structure_prepare mirrors deckhouse/tools/dmt-lint.sh: it folds the per-edition
# module trees (ee, be, fe, se, se-plus) into a single modules/ directory and
# extracts cloud providers into candi/cloud-providers, so the linter sees the
# same layout deckhouse lints in its own CI.
structure_prepare() {
  local modules_dir=("ee/modules" "ee/be/modules" "ee/fe/modules" "ee/se/modules" "ee/se-plus/modules")
  local cloud_providers_glob="030-cloud-provider-*"

  rm -rf "${WORK_DIR}"
  cp -R "${SRC_DIR}" "${WORK_DIR}"
  mkdir -p "${WORK_DIR}/candi/cloud-providers"
  mkdir -p "${WORK_DIR}/modules"

  local dir
  for dir in "${modules_dir[@]}"; do
    if [ -d "${WORK_DIR}/${dir}" ]; then
      cp -R "${WORK_DIR}/${dir}"/* "${WORK_DIR}/modules/" 2>/dev/null || true
    fi

    shopt -s nullglob
    local cloud_provider_dir
    for cloud_provider_dir in "${WORK_DIR}/${dir}/"${cloud_providers_glob}; do
      local cloud_provider_name
      cloud_provider_name=$(echo "${cloud_provider_dir}" | grep -oP '(?<=030-cloud-provider-)[^[:space:]]+')
      cp -R "${cloud_provider_dir}" "${WORK_DIR}/candi/cloud-providers/${cloud_provider_name}"
    done
    shopt -u nullglob
  done
}

echo "Preparing deckhouse module structure in ${WORK_DIR}"
structure_prepare

echo "Linting with: ${DMT_BIN}"
"${DMT_BIN}" --version || true

echo "Running: ${DMT_BIN} lint -l INFO ${WORK_DIR}/modules"
"${DMT_BIN}" lint -l INFO "${WORK_DIR}/modules"
