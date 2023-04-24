import { resolve } from "path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import { reactClickToComponent } from "vite-plugin-react-click-to-component";
import { crx, ManifestV3Export } from "@crxjs/vite-plugin";
import manifest from "./manifest.json";

export default defineConfig({
  plugins: [
    react(),
    reactClickToComponent(),
    crx({ manifest: manifest as ManifestV3Export }),
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