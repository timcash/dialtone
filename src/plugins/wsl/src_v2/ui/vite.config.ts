import { defineConfig } from "vite";
import { resolve } from "path";

export default defineConfig({
  root: ".",
  resolve: {
    alias: {
      "@ui": resolve(__dirname, "../../../../../libs/ui"),
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
  server: {
    port: 3000,
  },
});
