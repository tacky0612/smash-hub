import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// npm run dev でフロントエンドだけを起動する場合、/api はローカルのバックエンドへ転送する
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": process.env.API_PROXY_TARGET ?? "http://localhost:8081",
    },
  },
});
