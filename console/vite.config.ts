import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      // Dev convenience: forwards same-origin /api calls to a locally
      // port-forwarded control plane (kubectl port-forward svc/kubetraffic-control-plane 18080:8080).
      "/api": "http://localhost:18080",
    },
  },
})
