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
    // The palette test reads tokens.css as text (?raw); other CSS stays
    // stubbed.
    css: { include: [/tokens\.css/] },
    projects: [
      {
        extends: true,
        test: {
          name: "unit",
          environment: "node",
          include: ["src/**/*.test.{ts,tsx}"],
          exclude: ["src/**/*.dom.test.{ts,tsx}"],
        },
      },
      // Interaction tests (*.dom.test.tsx) fire events and check focus in a
      // simulated DOM (D42); the rest stay in node, which starts faster.
      {
        extends: true,
        test: {
          name: "dom",
          environment: "happy-dom",
          include: ["src/**/*.dom.test.{ts,tsx}"],
          setupFiles: ["src/test/dom-setup.ts"],
        },
      },
    ],
  },
});
