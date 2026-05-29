import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': { target: 'http://localhost:9090', changeOrigin: true, timeout: 30000 },
      '/ws': { target: 'ws://localhost:9090', ws: true, changeOrigin: true, timeout: 30000 },
    },
  },
})