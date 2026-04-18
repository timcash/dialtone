import { defineConfig } from 'vite';
import { resolve } from 'path';
import { readFileSync } from 'fs';

const pkg = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf-8'));

const proxyTarget = process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:8081';
const siteBasePath = process.env.VITE_SITE_BASE_PATH || '/';
const buildOutDir = process.env.VITE_BUILD_OUT_DIR || 'dist';
const cadAPIBaseURL = process.env.VITE_CAD_API_BASE_URL || '';

export default defineConfig({
  root: '.',
  base: siteBasePath,
  resolve: {
    alias: {
      '@ui': resolve(__dirname, '../../../ui/src_v1/ui'),
    },
  },
  define: {
    APP_VERSION: JSON.stringify(pkg.version),
    CAD_API_BASE_URL: JSON.stringify(cadAPIBaseURL),
  },
  build: {
    outDir: buildOutDir,
    emptyOutDir: true,
  },
  server: {
    port: 3012,
    host: true,
    allowedHosts: true,
    headers: {
      'Cache-Control': 'no-store',
    },
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
      },
      '/health': {
        target: proxyTarget,
        changeOrigin: true,
      },
    },
  },
});
