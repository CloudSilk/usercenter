/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// 开发期:Vite dev server(5173)代理所有后端前缀到 devserver(48180)。
// 生产:go:embed dist/ 由 Go 服务,无需 proxy。
const API_TARGET = process.env.UC_API_TARGET || 'http://localhost:48180'
const apiProxy = {
  target: API_TARGET,
  changeOrigin: true,
  secure: false,
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': apiProxy,
      '/admin/api': apiProxy,
      '/v1': apiProxy,
      '/oauth': apiProxy,
      '/scim': apiProxy,
      '/.well-known': apiProxy,
      '/metrics': apiProxy,
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: false,
    assetsDir: 'assets',
  },
  base: '/web/admin/',
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
  },
})
