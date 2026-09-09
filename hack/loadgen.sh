#!/usr/bin/env bash
# Fire a burst of requests at a target URL and print the status-code distribution.
#
# Usage:
#   hack/loadgen.sh [url]
#   COUNT=500 CONCURRENCY=20 hack/loadgen.sh http://localhost:8080/
#
# Requires a reachable target, e.g.:
#   kubectl -n demo port-forward svc/payment 8080:8080
set -euo pipefail

TARGET="${1:-http://localhost:8080/}"
COUNT="${COUNT:-200}"
CONCURRENCY="${CONCURRENCY:-10}"

if command -v hey >/dev/null; then
  exec hey -n "${COUNT}" -c "${CONCURRENCY}" "${TARGET}"
fi

echo "'hey' not found; falling back to a curl loop (${COUNT} requests, ${CONCURRENCY} parallel)"
tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT

seq "${COUNT}" | xargs -P "${CONCURRENCY}" -I_ \
  curl -s -o /dev/null -w '%{http_code}\n' "${TARGET}" >>"${tmp}"

echo "status code distribution:"
sort "${tmp}" | uniq -c | sort -rn
