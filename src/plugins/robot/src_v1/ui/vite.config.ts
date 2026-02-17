import { defineConfig } from 'vite';
import { readFileSync } from 'fs';
import { resolve } from 'path';

const pkg = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf-8'));

const proxyTarget = process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:8080';
const wsProxyTarget = proxyTarget.replace('http', 'ws');

export default defineConfig({
  root: '.',
  define: {
    APP_VERSION: JSON.stringify(pkg.version),
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: wsProxyTarget,
        ws: true,
      },
      '/nats-ws': {
        target: wsProxyTarget,
        ws: true,
      },
      '/api': proxyTarget,
      '/stream': proxyTarget,
    }
  },
  optimizeDeps: {
    include: ['nats.ws'],
  },
});
