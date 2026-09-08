import { defineConfig } from 'vite';
import solid from 'vite-plugin-solid';

const host = process.env.TAURI_DEV_HOST;

export default defineConfig(async () => ({
  plugins: [solid()],

  // Tauri uses fixed dev port; fail if not available
  clearScreen: false,
  server: {
    port: 5173,
    strictPort: true,
    host: host || false,
    hmr: host
      ? { protocol: 'ws', host, port: 1421 }
      : undefined,
    watch: {
      // Don't watch Rust source — Tauri handles it
      ignored: ['**/src-tauri/**'],
    },
  },

  // Env variables prefixed with VITE_ are exposed to the client
  envPrefix: ['VITE_', 'TAURI_ENV_*'],

  build: {
    target: 'es2021',
    minify: !process.env.TAURI_ENV_DEBUG ? 'esbuild' : false,
    sourcemap: !!process.env.TAURI_ENV_DEBUG,
  },
}));
