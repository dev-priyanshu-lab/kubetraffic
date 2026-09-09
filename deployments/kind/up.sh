#!/usr/bin/env bash
# Create the local kind cluster, load the sample-app image, deploy the demo.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-kubetraffic}"

for bin in kind kubectl docker; do
  command -v "${bin}" >/dev/null || { echo "error: '${bin}' not found in PATH" >&2; exit 1; }
done

if kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  echo "==> kind cluster '${CLUSTER_NAME}' already exists"
else
  echo "==> creating kind cluster '${CLUSTER_NAME}'"
  kind create cluster --config "${SCRIPT_DIR}/kind-cluster.yaml"
fi

kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
kubectl cluster-info --context "kind-${CLUSTER_NAME}"

"${SCRIPT_DIR}/load-images.sh"

echo "==> applying demo manifests"
kubectl apply -k "${REPO_ROOT}/demo/manifests"

echo "==> waiting for rollouts"
while read -r dep; do
  kubectl -n demo rollout status "${dep}" --timeout=120s
done < <(kubectl -n demo get deploy -o name)

echo
kubectl get pods -n demo -o wide
echo
echo "==> demo is up. Try:"
echo "    kubectl -n demo port-forward svc/payment 8080:8080"
echo "    curl -s localhost:8080/ | jq"
