import { defineConfig } from "vite";

export default defineConfig({
  server: {
    host: true,
    allowedHosts: true,
    cors: true,
    port: 5173,
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
