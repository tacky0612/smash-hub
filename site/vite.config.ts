import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// GitHub Pages（https://<owner>.github.io/<repo>/）で配信する。
// 画面の切り替えは HashRouter で行うため、相対パス（./）で出力すればリポジトリ名やフォークに依存しない。
export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
});
