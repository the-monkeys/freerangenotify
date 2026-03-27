import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    server: {
      port: 5173,
      host: "0.0.0.0",
      headers: {
        // Avoid flaky keep-alive socket reuse on some localhost stacks.
        Connection: "close",
      },
      watch: {
        usePolling: env.CHOKIDAR_USEPOLLING === "true",
        interval: Number(env.CHOKIDAR_INTERVAL || 1000),
        ignored: ["**/node_modules/**", "**/.git/**", "**/dist/**"],
      },
      proxy: {
        "/v1": {
          // API_PROXY_TARGET is server-side only (not exposed to browser)
          // Falls back to VITE_API_BASE_URL for local dev, then localhost
          target: env.API_PROXY_TARGET || env.VITE_API_BASE_URL || "http://localhost:8080",
          changeOrigin: true,
          secure: false,
        },
      },
    },
    build: {
      outDir: "dist",
    },
  }
    ;
});
