import { defineConfig } from 'vite';
import { readFileSync } from 'fs';
import { resolve } from 'path';

const pkg = JSON.parse(readFileSync(resolve(__dirname, 'package.json'), 'utf-8'));

export default defineConfig({
  root: '.',
  define: {
    APP_VERSION: JSON.stringify(pkg.version),
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 3013,
    host: true,
    allowedHosts: true,
    headers: {
      'Cache-Control': 'no-store',
    },
  },
});
