import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': { target: 'http://127.0.0.1:9090', changeOrigin: true },
      '/v1': { target: 'http://127.0.0.1:9090', changeOrigin: true },
      '/ws': { target: 'ws://127.0.0.1:9090', ws: true },
    },
  },
  build: {
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia'],
          naive: ['naive-ui'],
          charts: ['echarts'],
        },
      },
    },
  },
})
