import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig({
  resolve: {
    alias: {
      '@ui': resolve(__dirname, '../../../ui/src_v1/ui'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5181,
    strictPort: true,
    allowedHosts: true,
    cors: true,
  },
});
