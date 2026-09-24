#!/bin/sh
# Runs as an nginx docker-entrypoint.d hook, before nginx starts. Regenerates
# config.js from environment variables at container start, so the same built
# image can point at any operator's control-plane without a rebuild.
set -eu

CONFIG_FILE="/usr/share/nginx/html/config.js"
API_BASE_URL="${KUBETRAFFIC_API_BASE_URL:-}"

cat > "$CONFIG_FILE" <<EOF
window.__KUBETRAFFIC_CONFIG__ = {
  apiBaseUrl: "${API_BASE_URL}",
};
EOF
