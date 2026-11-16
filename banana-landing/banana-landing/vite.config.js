import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// dev-сервер фронта проксирует /api на Go-бэкенд (banana-backend, порт 8080),
// поэтому в браузере фронт и API выглядят как один origin — CORS не нужен.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080', '/healthz': 'http://localhost:8080' },
  },
  build: { outDir: 'dist' },
});
