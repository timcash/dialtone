import { defineConfig } from 'vite';
import { readFileSync } from 'fs';
import { resolve } from 'path';
import type { ProxyOptions } from 'vite';

const pkg = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf-8'));

const proxyTarget = process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:8080';
const wsProxyTarget = proxyTarget.replace(/^http:/, 'ws:').replace(/^https:/, 'wss:');
const proxyTargetURL = new URL(proxyTarget);
const proxyOrigin = proxyTargetURL.origin;

function wsProxyOptions(): ProxyOptions {
  return {
    target: wsProxyTarget,
    ws: true,
    changeOrigin: true,
    configure: (proxy) => {
      proxy.on('proxyReqWs', (proxyReq) => {
        proxyReq.setHeader('Origin', proxyOrigin);
      });
    },
  };
}

export default defineConfig({
  root: '.',
  resolve: {
    alias: {
      '@ui': resolve(__dirname, '../../../ui/src_v1/ui'),
    },
  },
  define: {
    APP_VERSION: JSON.stringify(pkg.version),
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  server: {
    port: 3000,
    allowedHosts: [
      'legion-wsl-1.shad-artichoke.ts.net',
      '.shad-artichoke.ts.net',
    ],
    headers: {
      'Cache-Control': 'no-store',
    },
    proxy: {
      '/ws': wsProxyOptions(),
      '/nats-ws': wsProxyOptions(),
      '/natsws': wsProxyOptions(),
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/stream': {
        target: proxyTarget,
        changeOrigin: true,
      },
    }
  },
  optimizeDeps: {
    include: ['nats.ws'],
  },
});
