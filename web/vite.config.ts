import { fileURLToPath, URL } from "node:url";
import { paraglideVitePlugin } from "@inlang/paraglide-js";
import tailwindcss from "@tailwindcss/vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const backend = process.env.MAHLZEIT_BACKEND ?? "http://localhost:8080";

export default defineConfig({
  plugins: [
    tanstackRouter({ target: "react", autoCodeSplitting: true }),
    react(),
    tailwindcss(),
    paraglideVitePlugin({
      project: "./project.inlang",
      outdir: "./src/paraglide",
      strategy: ["localStorage", "preferredLanguage", "baseLocale"],
    }),
  ],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    proxy: { "/api": backend, "/auth": backend, "/healthz": backend },
  },
  build: { sourcemap: false, chunkSizeWarningLimit: 800 },
  test: { include: ["src/**/*.test.ts"], environment: "node" },
});
