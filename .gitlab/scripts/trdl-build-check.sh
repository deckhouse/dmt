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

# Reproduces the trdl server release build from trdl.yaml, without a tag.
#
# The trdl server clones the tag into memory (tracked files only, no .git) and
# builds it with a generated Dockerfile
# (werf/trdl: server/pkg/docker/dockerfile.go):
#
#   FROM <docker_image> AS builder
#   COPY . /git
#   WORKDIR /git
#   RUN mkdir -p /result
#   RUN <commands joined with &&>
#   FROM scratch
#   COPY --from=builder /result /result/
#
# This script generates the same Dockerfile and runs `docker build` over the
# same context. If this build passes, the release build for a future tag
# passes too.
#
# Usage:
#   trdl-build-check.sh [--dry-run] [tag]
#
#   --dry-run  print the generated Dockerfile and exit
#   tag        synthetic release tag, default v0.0.0-trdl-check

set -euo pipefail

dry_run=0
if [ "${1:-}" = "--dry-run" ]; then
  dry_run=1
  shift
fi
tag="${1:-v0.0.0-trdl-check}"

cd "$(dirname "$0")/../.."

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

# Same context as trdl: tracked files only. Uncommitted changes to tracked
# files are included, so the check can be run locally before committing.
ctx="$workdir/context"
mkdir -p "$ctx"
rev="$(git stash create)"
if [ -n "$rev" ]; then
  echo "note: building HEAD plus uncommitted changes to tracked files" >&2
fi
git archive "${rev:-HEAD}" | tar -x -C "$ctx"

# trdl.yaml is flat: a scalar image and a plain list of commands. Keep it that
# way - this parser does not understand anything else. trdl accepts both
# dockerImage and the legacy docker_image spelling.
image="$(awk '$1 == "dockerImage:" || $1 == "docker_image:" {print $2; exit}' "$ctx/trdl.yaml")"
commands="$(grep '^ *- ' "$ctx/trdl.yaml" \
  | sed -e 's/^ *- //' -e "s/{{ *\.Tag *}}/${tag}/g" \
  | awk 'NR > 1 {printf " && "} {printf "%s", $0} END {print ""}')"

if [ -z "$image" ] || [ -z "$commands" ]; then
  echo "failed to parse docker_image/commands from trdl.yaml" >&2
  exit 1
fi

mkdir -p "$ctx/.trdl"
cat > "$ctx/.trdl/Dockerfile" <<EOF
FROM ${image} AS builder
COPY . /git
WORKDIR /git
RUN mkdir -p /result
RUN ${commands}
FROM scratch
COPY --from=builder /result /result/
EOF

if [ "$dry_run" = 1 ]; then
  cat "$ctx/.trdl/Dockerfile"
  exit 0
fi

DOCKER_BUILDKIT=1 docker build \
  --platform linux/amd64 \
  --file "$ctx/.trdl/Dockerfile" \
  --output "type=local,dest=$workdir/out" \
  --progress plain \
  "$ctx"

echo
echo "Artifacts built for ${tag}:"
ls -lR "$workdir/out/result"
test -n "$(ls -A "$workdir/out/result")"

# The tag reaches the binary through -ldflags in scripts/ci/build_release.sh.
if [ "$(uname -sm)" = "Linux x86_64" ]; then
  "$workdir/out/result/linux-amd64/bin/dmt" --version | grep -F "$tag"
fi
