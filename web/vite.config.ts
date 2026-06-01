import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:9090', changeOrigin: true },
      '/ws': { target: 'ws://localhost:9090', ws: true },
    },
  },
})
