#!/usr/bin/env bash
# Generate Go stubs for proto/kubetraffic/v1/route.proto into
# controller/internal/grpcapi/kubetrafficv1/.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTROLLER_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${CONTROLLER_DIR}/.." && pwd)"
BIN="${CONTROLLER_DIR}/bin"

mkdir -p "${BIN}"
command -v protoc >/dev/null || { echo "error: protoc not found (brew install protobuf)" >&2; exit 1; }

GOBIN="${BIN}" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.34.2
GOBIN="${BIN}" go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

OUT="${CONTROLLER_DIR}/internal/grpcapi/kubetrafficv1"
mkdir -p "${OUT}"

PATH="${BIN}:${PATH}" protoc \
  --proto_path="${REPO_ROOT}/proto" \
  --go_out="${CONTROLLER_DIR}" --go_opt=module=github.com/kubetraffic/controller \
  --go-grpc_out="${CONTROLLER_DIR}" --go-grpc_opt=module=github.com/kubetraffic/controller \
  "${REPO_ROOT}/proto/kubetraffic/v1/route.proto"

echo "generated into ${OUT}"
