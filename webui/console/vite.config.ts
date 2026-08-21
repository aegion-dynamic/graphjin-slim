import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vite-plus"
import path from "node:path"

// Vite+ unifies dev/build plus Oxlint and Oxfmt configuration in this file.
// The production bundle is emitted into the Go module's embedded assets so
// `webui.Handler` serves exactly what this build produced.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  // Oxfmt: project style — no semicolons, always double quotes.
  fmt: {
    semi: false,
    singleQuote: false,
  },
  build: {
    outDir: "../assets/build",
    emptyOutDir: true,
    sourcemap: false,
  },
})
