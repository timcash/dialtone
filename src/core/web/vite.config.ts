import { defineConfig } from 'vite';

export default defineConfig({
    server: {
        proxy: {
            '/api': {
                target: 'http://127.0.0.1:8080',
                changeOrigin: true,
                secure: false,
            },
            '/stream': {
                target: 'http://127.0.0.1:8080',
                changeOrigin: true,
                secure: false,
                ws: true,
            }
        }
    }
});
