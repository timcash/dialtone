import { defineConfig } from 'vite';

export default defineConfig({
  root: '.',
  build: {
    outDir: 'dist',
    emptyOutDir: true
  },
  server: {
    port: 3000
  },
  optimizeDeps: {
    include: ['nats.ws'],
  },
});
