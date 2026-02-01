import { defineConfig } from 'vite';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
    plugins: [
        {
            name: 'html-rewrite',
            configureServer(server) {
                server.middlewares.use((req, res, next) => {
                    if (req.url?.startsWith('/about')) {
                        req.url = '/src/pages/about.html';
                    } else if (req.url?.startsWith('/docs')) {
                        req.url = '/src/pages/docs.html';
                    }
                    next();
                });
            }
        }
    ],
    appType: 'mpa',
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
                about: resolve(__dirname, 'src/pages/about.html'),
                docs: resolve(__dirname, 'src/pages/docs.html'),
            },
        },
    },
});
