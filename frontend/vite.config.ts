import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../internal/webui/assets/generated",
    emptyOutDir: true,
    rollupOptions: {
      // Singleserve serves its browser client; it is never bundled.
      external: ["singleserve-client"],
      output: {
        paths: {
          "singleserve-client": "/_singleserve/client.js",
        },
      },
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
