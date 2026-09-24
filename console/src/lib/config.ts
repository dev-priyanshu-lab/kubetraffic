declare global {
  interface Window {
    __KUBETRAFFIC_CONFIG__?: {
      apiBaseUrl?: string;
    };
  }
}

/**
 * The control-plane's REST API base URL, resolved at runtime (not build time) so the
 * same console image works against any operator's deployment. See public/config.js
 * and docker-entrypoint.sh.
 */
export function apiBaseUrl(): string {
  return window.__KUBETRAFFIC_CONFIG__?.apiBaseUrl ?? "";
}
