import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

const backendUrl = process.env.VITE_BACKEND_URL || 'http://localhost:9090'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': path.resolve(__dirname, 'src') },
  },
  server: {
    port: 5173,
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: backendUrl,
        changeOrigin: true,
        timeout: 30000,
        configure: (proxy) => {
          proxy.on('error', (err) => {
            console.log('[proxy] backend not reachable:', err.message)
          })
        },
      },
      '/ws': {
        target: backendUrl,
        ws: true,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['vue', 'vue-router', 'pinia', 'axios'],
          ui: ['naive-ui'],
          charts: ['echarts', 'vue-echarts'],
        },
      },
    },
  },
})
