import { defineConfig, loadEnv } from "vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const backendTarget = env.TEST_UI_BACKEND_ORIGIN || "http://127.0.0.1:8787";

  return {
    server: {
      host: true,
      allowedHosts: true,
      cors: true,
      port: 5174,
      proxy: {
        "/api": {
          target: backendTarget,
          changeOrigin: true,
        },
        "/stream": {
          target: backendTarget,
          changeOrigin: true,
        },
        "/natsws": {
          target: backendTarget,
          changeOrigin: true,
          ws: true,
        },
      },
    },
    build: {
      outDir: "dist",
      emptyOutDir: true,
    },
  };
});
