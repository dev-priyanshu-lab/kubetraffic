// Runtime configuration for the KubeTraffic console.
//
// In the published container image this file is regenerated at startup (see
// console/docker-entrypoint.sh) from the KUBETRAFFIC_API_BASE_URL environment
// variable, so the SAME built image works against any operator's control-plane
// without rebuilding. This checked-in copy is only the dev-server default.
//
// An empty string means "same origin as the console" (e.g. behind a reverse
// proxy that forwards /api to the control-plane).
window.__KUBETRAFFIC_CONFIG__ = {
  apiBaseUrl: "",
};
