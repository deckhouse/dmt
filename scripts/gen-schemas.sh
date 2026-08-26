#!/usr/bin/env bash
#
# Copyright 2026 Flant JSC
#
# Regenerates the embedded schema catalog (internal/schemas/data/schemas.tar.gz)
# that dmt uses to validate rendered manifests against their JSON schemas.
#
# Sources:
#   - third-party CRD schemas: https://github.com/datreeio/crds-catalog
#   - built-in Kubernetes type schemas: https://github.com/yannh/kubernetes-json-schema
#
# Usage:
#   scripts/gen-schemas.sh [k8s-version]
#
# Environment:
#   K8S_VERSION   Kubernetes schema version to embed (default: v1.30.0)
#   DATREE_REF    datree/crds-catalog git ref (default: main)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
K8S_VERSION="${1:-${K8S_VERSION:-v1.30.0}}"
DATREE_REF="${DATREE_REF:-main}"
OUT="${REPO_ROOT}/internal/schemas/data/schemas.tar.gz"

WORK="$(mktemp -d)"
trap 'rm -rf "${WORK}"' EXIT

echo ">> fetching datree/crds-catalog (${DATREE_REF})"
git clone --quiet --depth 1 --branch "${DATREE_REF}" \
	https://github.com/datreeio/crds-catalog.git "${WORK}/datree"

echo ">> fetching kubernetes JSON schemas (${K8S_VERSION}-standalone-strict)"
git clone --quiet --depth 1 --filter=blob:none --sparse \
	https://github.com/yannh/kubernetes-json-schema.git "${WORK}/k8s"
git -C "${WORK}/k8s" sparse-checkout set "${K8S_VERSION}-standalone-strict"

echo ">> packing catalog -> ${OUT}"
go run "${REPO_ROOT}/tools/schemagen" \
	-datree "${WORK}/datree" \
	-k8s "${WORK}/k8s/${K8S_VERSION}-standalone-strict" \
	-out "${OUT}"

echo ">> done: $(du -h "${OUT}" | cut -f1) ${OUT}"
