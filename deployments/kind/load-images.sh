#!/usr/bin/env bash
# Build the sample-app image and load it into the kind node cache.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-kubetraffic}"
IMAGE="${IMAGE:-kubetraffic/sample-app:dev}"

echo "==> building ${IMAGE}"
# --provenance=false keeps this a plain single-platform image (no OCI attestation
# index) so `kind load docker-image` imports it cleanly.
docker build --provenance=false -t "${IMAGE}" "${REPO_ROOT}/demo/sample-app"

echo "==> loading ${IMAGE} into kind cluster '${CLUSTER_NAME}'"
kind load docker-image "${IMAGE}" --name "${CLUSTER_NAME}"
