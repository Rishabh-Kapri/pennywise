import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';
import tailwindcss from '@tailwindcss/vite';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    // Use one loopback address so localhost cannot reach a different IPv4
    // server while Vite is listening only on IPv6. Compose overrides --host.
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: process.env.DEV_API_PROXY ? {
      "/api": {
        target: process.env.DEV_API_PROXY, changeOrigin: true, ws: true,
        // Browser requests are same-origin here; the private API sees the proxy.
        // This also supports dynamically allocated worktree ports.
        configure: (proxy) => {
          proxy.on('proxyReq', (request) => request.removeHeader('origin'));
        },
      },
    } : undefined,
    headers: {
      'Cross-Origin-Opener-Policy': 'same-origin-allow-popups',
      'Cross-Origin-Embedder-Policy': 'unsafe-none',
    },
  },
});
