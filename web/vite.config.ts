import { defineConfig } from "vite";
import solid from "vite-plugin-solid";
import path from "path";

export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: [
      { find: "@/lib/utils", replacement: path.resolve(__dirname, "./src/lib/utils.ts") },
      { find: "@/lib", replacement: path.resolve(__dirname, "./lib") },
      { find: "@", replacement: path.resolve(__dirname, "./src") },
    ],
  },
});
