import { defineConfig } from "vite";
import path from "path";

export default defineConfig({
  server: {
    host: true,
    allowedHosts: true,
    cors: true,
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  resolve: {
    alias: {
      "../../../../ui": path.resolve(__dirname, "../../../../ui"),
    },
  },
});
