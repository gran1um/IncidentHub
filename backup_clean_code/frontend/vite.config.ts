import { defineConfig } from 'vite';
import path from 'path';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { configDefaults } from "vitest/config";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./tests/unit/setup.ts",
    css: true,
    exclude: [...configDefaults.exclude, "**/tests/e2e/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary", "lcov"],
      include: [
        "src/lib/i18n.ts",
        "src/pages/alerts.tsx",
        "src/pages/cases.tsx",
        "src/pages/administration.tsx"
      ],
      thresholds: {
        lines: 60,
        functions: 10,
        statements: 60,
        branches: 50,
      },
    },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/healthz': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/swagger': {
        target: 'http://localhost:8080',
        changeOrigin: true
      },
      '/dev/swagger': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
});
