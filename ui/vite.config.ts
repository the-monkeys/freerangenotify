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
      port: 3000,
      host: "0.0.0.0",
      allowedHosts: true,
      watch: {
        usePolling: process.env.CHOKIDAR_USEPOLLING === "true" || env.CHOKIDAR_USEPOLLING === "true",
        interval: Number(process.env.CHOKIDAR_INTERVAL || env.CHOKIDAR_INTERVAL || 1000),
        ignored: ["**/node_modules/**", "**/.git/**", "**/dist/**"],
      },
      proxy: {
        "/v1": {
          target: process.env.API_PROXY_TARGET || env.API_PROXY_TARGET || env.VITE_API_BASE_URL || "http://localhost:8080",
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
