import { fileURLToPath, URL } from 'node:url'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

function manualChunks(id: string) {
  const normalizedId = id.replaceAll('\\', '/')
  if (!normalizedId.includes('/node_modules/')) return undefined

  if (/\/node_modules\/(?:@vue|vue|vue-router|pinia)\//.test(normalizedId)) {
    return 'vue-core'
  }
  if (normalizedId.includes('/node_modules/element-plus/')) {
    return 'element-plus'
  }
  if (/\/node_modules\/(?:xlsx|xlsx-js-style|cfb|ssf|crc-32|adler-32|codepage|wmf|word)\//.test(normalizedId)) {
    return 'spreadsheet'
  }
  if (normalizedId.includes('/node_modules/axios/')) {
    return 'http-client'
  }
  return 'vendor'
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/health': 'http://localhost:8080',
    },
  },
  build: {
    chunkSizeWarningLimit: 850,
    rollupOptions: {
      output: { manualChunks },
    },
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
      reportsDirectory: './coverage',
      include: ['src/views/RoomScreenView.vue'],
    },
  },
})
