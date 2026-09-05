import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: path.resolve(import.meta.dirname, "../cmd/panel-core/webdist"),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        ws: true,
      },
    },
  },
});
