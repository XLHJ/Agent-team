/*
 * ApexForge (https://github.com/MrXujiang/ApexForge) — vendored sub-app.
 * License: GPL-3.0 (see LICENSE). Modified for embedding into the Agent++
 * control panel: build/dev base is "/apexforge-app/" (dev) or relative
 * (build, for Electron file:// loading) and the dev port moved to 5175
 * to avoid conflicts with the other vendored sub-apps.
 */
import react from '@vitejs/plugin-react';
import path from 'node:path';
import { defineConfig } from 'vite';

export default defineConfig(({ command }) => ({
  plugins: [react()],
  base: process.env.VITE_BASE || (command === 'build' ? './' : '/apexforge-app/'),
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5175,
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom'],
          three: ['three'],
        },
      },
    },
  },
}));
