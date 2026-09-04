import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import { wgslVitePlugin } from "@vgpu/wgsl/loader-vite";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const apiToken = env.VITE_API_TOKEN?.trim();
  const proxyHeaders: Record<string, string> = {};
  if (apiToken) {
    proxyHeaders.Authorization = `Bearer ${apiToken}`;
  }

  return {
    plugins: [react(), wgslVitePlugin()],
    test: {
      environment: "node",
    },
    server: {
      port: 5173,
      proxy: {
        "/api": {
          target: "http://localhost:8080",
          changeOrigin: true,
          headers: proxyHeaders,
        },
        "/chat": {
          target: "http://localhost:8090",
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/chat/, ""),
        },
      },
    },
  };
});
