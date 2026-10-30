import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// https://vitejs.dev/config/
// VSCode webview requires relative asset paths (base: './') and
// outputs to ../dist/webview/ so the extension host can load index.html
// from disk via fs.readFileSync (per RFC 2026-10-18 §2.4).
export default defineConfig({
  base: './',
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: path.resolve(__dirname, '../dist/webview'),
    emptyOutDir: true,
  },
})
