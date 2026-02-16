import { defineConfig } from 'vite';

export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://127.0.0.1:8080',
        ws: true,
      },
      '/api': 'http://127.0.0.1:8080',
      '/stream': 'http://127.0.0.1:8080',
    }
  },
  optimizeDeps: {
    include: ['nats.ws'],
  },
});
