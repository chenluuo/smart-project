import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:18080',
      '/agent': 'http://localhost:8000',
      '/actuator': 'http://127.0.0.1:18080'
    }
  }
});
