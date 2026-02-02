import { defineConfig } from 'vite';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
    plugins: [],
    appType: 'spa',
    server: {
        host: '127.0.0.1',
        proxy: {
            '/api/cad': {
                target: 'http://127.0.0.1:8081',
                changeOrigin: true,
            }
        }
    },
    build: {
        rollupOptions: {
            input: {
                main: resolve(__dirname, 'index.html'),
            },
        },
    },
});
