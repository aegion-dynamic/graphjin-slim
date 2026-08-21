import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// The production bundle is emitted into the Go module's embedded assets so
// `webui.Handler` serves exactly what this build produced.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../assets/build',
    emptyOutDir: true,
    sourcemap: false,
  },
});
