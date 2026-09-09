#!/usr/bin/env bash
# Delete the local kind cluster.
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-kubetraffic}"
command -v kind >/dev/null || { echo "error: 'kind' not found in PATH" >&2; exit 1; }

if kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  kind delete cluster --name "${CLUSTER_NAME}"
else
  echo "kind cluster '${CLUSTER_NAME}' does not exist; nothing to do"
fi
