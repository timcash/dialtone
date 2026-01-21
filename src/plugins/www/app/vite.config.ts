import { defineConfig } from 'vite';
import { resolve } from 'path';

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
