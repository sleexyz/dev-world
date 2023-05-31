import { resolve } from "path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import { reactClickToComponent } from "vite-plugin-react-click-to-component";
import pluginRewriteAll from 'vite-plugin-rewrite-all';

export default defineConfig({
  plugins: [
    react(),
    reactClickToComponent(),
    pluginRewriteAll(),
  ],
  build: {
    rollupOptions: {
      input: {
        index: resolve(__dirname, "index.html"),
      },
    },
  },
  server: {
    port: 12344,
    strictPort: true,
  },
  clearScreen: false,
});
